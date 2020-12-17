package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
)

type Params struct {
	ImageWidth  int
	ImageHeight int
}

var (
	globalWorld [][]byte
	p           Params
)

func makeMatrix(height, width int) [][]byte {
	matrix := make([][]byte, height)
	for i := range matrix {
		matrix[i] = make([]byte, width)
	}
	return matrix
}

//Take in one part of the world and do the relating operatinos
func worker(address string, name string, StartX int, EndX int, StartY int, EndY int, world [][]byte, outWorld chan [][]byte, outCell chan []Cell) {
	client, _ := rpc.Dial("tcp", address)

	calRequest := CalRequest{StartY, EndY, StartX, EndX, globalWorld}
	calResponse := new(CalResponse)
	err2 := client.Call(name, calRequest, calResponse)
	if err2 != nil {
		fmt.Println("Func of Subserver Return Error")
		fmt.Println(err2)
		fmt.Println("Closing subscriber thread.")
	}

	outWorld <- calResponse.World
	outCell <- calResponse.FlipCell
}

type Broker struct{}

func (b *Broker) WorldTransfer(req WorldRequest, res *WorldResponse) (err error) {
	globalWorld = req.World
	p.ImageHeight = req.Height
	p.ImageWidth = req.Width
	res.Flag = true
	return
}

func (b *Broker) Subscribe(req SubRequest, res *SubResponse) (err error) {
	worldCell := make([]chan []Cell, 4)
	out := make([]chan [][]byte, 4)
	workerHeight := p.ImageHeight / 4
	address := req.FactoryAddress
	for i := range out {
		out[i] = make(chan [][]byte)
	}

	for i := 0; i < 3; i++ {
		go worker(address[i], req.FuncName, 0, p.ImageWidth, i*workerHeight, (i+1)*workerHeight, globalWorld, out[i], worldCell[i])
	}
	go worker(address[3], req.FuncName, 0, p.ImageWidth, 0, p.ImageHeight, globalWorld, out[3], worldCell[3])

	newWorld := makeMatrix(0, 0)
	for j := 0; j < 4; j++ {
		part := <-out[j]
		newWorld = append(newWorld, part...)
	}

	var newCell []Cell
	for i := 0; i < 4; i++ {
		part := <-worldCell[i]
		newCell = append(newCell, part...)
	}

	globalWorld = newWorld
	res.World = newWorld
	res.FlipCell = newCell
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Broker{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
