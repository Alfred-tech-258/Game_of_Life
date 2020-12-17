package gol

var NextState = "RemoteCalculate.CalOneTurn"

type Cell struct {
	Y, X int
}

type WorldRequest struct {
	World         [][]byte
	Height, Width int
}

type WorldResponse struct {
	Flag bool
}

type SubRequest struct {
	FactoryAddress []string
	FuncName       string
}

type SubResponse struct {
	World    [][]byte
	FlipCell []Cell
}

type CalRequest struct {
	Sy, Ey, Sx, Ex int
	World          [][]byte
}

type CalResponse struct {
	World    [][]byte
	FlipCell []Cell
}

type Statuts struct {
	Kstatus bool
}

type Kquitting struct {
	Kkey string
}
