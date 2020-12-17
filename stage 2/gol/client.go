package gol

import (
	"fmt"
	"net/rpc"
	"strconv"
	"strings"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events    chan<- Event
	ioCommand chan<- ioCommand
	ioIdle    <-chan bool

	input    <-chan uint8
	output   chan<- uint8
	filename chan<- string

	keyPresses <-chan rune
}

func handleErr(err error) {
	fmt.Println("RPC client returned error:")
	fmt.Println(err)
	fmt.Println("Shutting down client.")
	return
}

func makeMatrix(height, width int) [][]byte {
	matrix := make([][]byte, height)
	for i := range matrix {
		matrix[i] = make([]byte, width)
	}
	return matrix
}

func getIamage(p Params, c distributorChannels) [][]byte {
	c.ioCommand <- ioInput
	c.filename <- strings.Join([]string{strconv.Itoa(p.ImageWidth), strconv.Itoa(p.ImageHeight)}, "x")

	world := makeMatrix(p.ImageHeight, p.ImageWidth)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			val := <-c.input
			world[y][x] = val
			if world[y][x] == 255 {
				c.events <- CellFlipped{CompletedTurns: 0, Cell: util.Cell{X: x, Y: y}}
			}
		}
	}
	return (world)
}

func makeImmutableWorld(matrix [][]byte) func(y, x int) byte {
	return func(y, x int) byte {
		return matrix[y][x]
	}
}

func printImage(c distributorChannels, p Params, turn int, world [][]byte) {
	c.ioCommand <- ioOutput
	c.filename <- strings.Join([]string{strconv.Itoa(p.ImageWidth), strconv.Itoa(p.ImageHeight), strconv.Itoa(p.Turns)}, "x")

	for m := 0; m < p.ImageHeight; m++ {
		for n := 0; n < p.ImageWidth; n++ {
			c.output <- world[m][n]
		}
	}

	c.events <- ImageOutputComplete{
		CompletedTurns: turn,
		Filename:       strings.Join([]string{strconv.Itoa(p.ImageWidth), strconv.Itoa(p.ImageHeight), strconv.Itoa(p.Turns)}, "x")}
}

func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	aliveCells := []util.Cell{}

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}

func distributor(p Params, c distributorChannels) {
	world := getIamage(p, c)
	ticker := time.NewTicker(2 * time.Second)

	factoryAddr := make([]string, 4)
	workerAddr := "127.0.0.1:8030"
	client, _ := rpc.Dial("tcp", workerAddr)
	defer client.Close()

	turn := 0

	worldRequest := WorldRequest{world, p.ImageHeight, p.ImageWidth}
	worldResponse := new(WorldResponse)
	err := client.Call("Broker.WorldTransfer", worldRequest, worldResponse)
	if err != nil {
		handleErr(err)
	}

	for turn < p.Turns {
		factoryAddr[0] = "127.0.0.1:8040"
		factoryAddr[1] = "127.0.0.1:8050"
		factoryAddr[2] = "127.0.0.1:8060"
		factoryAddr[3] = "127.0.0.1:8070"
		subRequest := SubRequest{factoryAddr, NextState}
		subResponse := new(SubResponse)
		err := client.Call("Broker.Subscribe", subRequest, subResponse)
		if err != nil {
			handleErr(err)
		}

		world = subResponse.World
		needFlip := subResponse.FlipCell
		for _, cell := range needFlip {
			c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: cell.X, Y: cell.Y}}
		}

		c.events <- TurnComplete{turn}
		turn++

		token := false
		select {
		case <-ticker.C:
			c.events <- AliveCellsCount{
				CompletedTurns: turn,
				CellsCount:     len(calculateAliveCells(p, world))}
		case command := <-c.keyPresses:
			switch command {
			case 's':
				c.events <- StateChange{turn, Executing}
				printImage(c, p, turn, world)
			case 'p':
				c.events <- StateChange{turn, Paused}
				printImage(c, p, turn, world)
				check := false
				for {
					key := <-c.keyPresses
					if key == 'p' {
						c.events <- StateChange{turn, Executing}
						fmt.Println("Countinuing")
						c.events <- TurnComplete{turn}
						check = true
					}
					if check {
						break
					}
				}
			case 'q':
				c.events <- StateChange{turn, Quitting}
				printImage(c, p, turn, world)
				c.events <- TurnComplete{turn}
				token = true
			case 'k':
				printImage(c, p, turn, world)
				subRequest := SubRequest{factoryAddr, "RemoteCalculate.Exit"}
				subResponse := new(SubResponse)
				client.Call("Broker.Subscribe", subRequest, subResponse)
				fmt.Println("Completely closed server!")
			}

		default:
		}
		if token == true {
			break
		}
	}

	c.ioCommand <- ioOutput
	c.filename <- strings.Join([]string{strconv.Itoa(p.ImageWidth), strconv.Itoa(p.ImageHeight), strconv.Itoa(p.Turns)}, "x")

	for m := 0; m < p.ImageHeight; m++ {
		for n := 0; n < p.ImageWidth; n++ {
			c.output <- world[m][n]
		}
	}

	// Make sure that the Io has finished any output before exiting.
	c.events <- ImageOutputComplete{
		CompletedTurns: turn,
		Filename:       strings.Join([]string{strconv.Itoa(p.ImageWidth), strconv.Itoa(p.ImageHeight), strconv.Itoa(p.Turns)}, "x")}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: calculateAliveCells(p, world)}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
