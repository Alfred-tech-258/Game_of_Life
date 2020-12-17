package main

import (
	"flag"
	"net"
	"net/rpc"
	"os"
)

type Params struct {
	ImageHeight int
	ImageWidth  int
}

const alive = 255
const dead = 0

var p Params

func mod(x, m int) int {
	return (x + m) % m
}

func makeMatrix(height, width int) [][]byte {
	matrix := make([][]byte, height)
	for i := range matrix {
		matrix[i] = make([]byte, width)
	}
	return matrix
}

func makeImmutableWorld(matrix [][]byte) func(y, x int) byte {
	return func(y, x int) byte {
		return matrix[y][x]
	}
}

func calculateNeighbours(y, x int, world func(y, x int) byte) int {
	neighbours := 0
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			if i != 0 || j != 0 {
				if world(mod(y+i, p.ImageHeight), mod(x+j, p.ImageWidth)) == alive {
					neighbours++
				}
			}
		}
	}
	return neighbours
}

func calculateNextState(StartY int, EndY int, StartX int, EndX int, world func(y, x int) byte) CalResponse {
	newWorld := makeMatrix(EndY-StartY, EndX-StartX)
	var flipCell []Cell
	for y := 0; y < EndY-StartY; y++ {
		Y := y + StartY

		for x := 0; x < EndX-StartX; x++ {
			neighbours := calculateNeighbours(Y, x, world)
			if world(Y, x) == alive {
				if neighbours == 2 || neighbours == 3 {
					newWorld[y][x] = alive
				} else {
					newWorld[y][x] = dead
					flipCell = append(flipCell, Cell{Y, x})
				}
			}

			if world(Y, x) == 0 {
				if neighbours == 3 {
					newWorld[y][x] = alive
					flipCell = append(flipCell, Cell{Y, x})
				} else {
					newWorld[y][x] = dead
				}
			}
		}
	}
	return CalResponse{newWorld, flipCell}
}

type RemoteCalculate struct{}

func (r *RemoteCalculate) CalOneTurn(req CalRequest, res *CalResponse) (err error) {
	localWorld := req.World
	p.ImageHeight = req.Ey
	p.ImageWidth = req.Ex
	remoteWorld := makeImmutableWorld(localWorld)
	result := calculateNextState(req.Sy, req.Ey, 0, p.ImageWidth, remoteWorld)
	res.World = result.World
	res.FlipCell = result.FlipCell
	return
}

func (r *RemoteCalculate) Exit(status Statuts, quit *Kquitting) (err error) {
	os.Exit(0)
	return
}

func main() {
	pAddr := flag.String("ip", "127.0.0.1:8050", "IP and port to listen on")
	flag.Parse()
	rpc.Register(&RemoteCalculate{})
	listener, _ := net.Listen("tcp", *pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
