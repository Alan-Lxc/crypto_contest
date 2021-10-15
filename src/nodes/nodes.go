package nodes

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	//"fmt"
	"github.com/Alan-Lxc/crypto_contest/src/basic/commitment"
	"github.com/Alan-Lxc/crypto_contest/src/basic/interpolation"
	"github.com/Alan-Lxc/crypto_contest/src/basic/point"
	"github.com/Alan-Lxc/crypto_contest/src/basic/poly"
	"github.com/Nik-U/pbc"
	"github.com/golang/protobuf/proto"
	"math/big"
	//"github.com/Nik-U/pbc"
	pb "github.com/Alan-Lxc/crypto_contest/src/service"
	"github.com/ncw/gmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

type Node struct {
	//Label of Node
	label int
	//Total number of Nodes
	counter int
	//Degree of polynomial
	degree int
	//the polynomial was set on Z_p
	p *gmp.Int
	// Rand source
	randState *rand.Rand

	//Polynomial State
	secretShares []*point.Point

	//To store the point(shares) sent from other node
	recPoint []*point.Point
	//To recode the share that have already received
	recCounter *int
	//The poly reconstructed with the shares
	recPoly *poly.Poly
	//Mutex to control
	mutex sync.Mutex

	// Proactivization Polynomial
	proPoly *poly.Poly
	//New Polynomials after phase3
	newPoly *poly.Poly

	// Lagrange Coefficient
	lambda []*gmp.Int
	//	0Shares
	_0Shares     []*gmp.Int
	_0ShareSum   *gmp.Int
	_0ShareCount *int
	//Commitment & Witness in Phase 2
	zeroShareCmt *pbc.Element
	zeroPolyCmt  *pbc.Element
	zeroPolyWit  *pbc.Element
	// Counter for New Secret Shares
	shareCnt *int

	//the other nodes in the committee
	Client []*Node

	////Secret shares of node p(a0,y)
	//secretShare []*point.Point
	//IP_address of node
	ipAddress []string
	//board IP address
	ipOfBoard string
	//clientconn
	clientConn []*grpc.ClientConn
	//nodeService
	nodeService []pb.NodeServiceClient
	//boardconn
	boardConn *grpc.ClientConn
	//boardService
	boardService pb.BulletinBoardServiceClient

	// Commitment and Witness from BulletinBoard
	oldPolyCmt      []*pbc.Element
	zerosumShareCmt []*pbc.Element
	zerosumPolyCmt  []*pbc.Element
	zerosumPolyWit  []*pbc.Element
	midPolyCmt      []*pbc.Element

	newPolyCmt []*pbc.Element

	// [+] Commitment
	dc  *commitment.DLCommit
	dpc *commitment.DLPolyCommit

	// Metrics
	totMsgSize   *int
	metadataPath string "github.com/Alan-Lxc/crypto_contest/src/basic/poly"
	// Initialize Flag
	iniflag *bool
	s1      *time.Time
	e1      *time.Time
	s2      *time.Time
	e2      *time.Time
	s3      *time.Time
	e3      *time.Time
}

func (node *Node) Phase1Getstart(ctx context.Context, msg *pb.RequestMsg) (*pb.ResponseMsg, error) {
	panic("implement me")
}

func (node *Node) Phase3Sending(ctx context.Context, msg *pb.PointMsg) (*pb.ResponseMsg, error) {
	panic("implement me")
}

func (node *Node) GetLabel() int {
	if node != nil {
		return node.label
	} else {
		return 0
	}

}

//func (node *Node) connect(ptrs []*Node) {
//	for i := 0; i < node.counter; i++ {
//
//		if i != node.label-1 {
//			node.Client[i] ="github.com/Alan-Lxc/crypto_contest/src/basic/poly" ptrs[i]
//		}
//	}
//}

//Server Handler
func (node *Node) Phase1GetStart(ctx context.Context, msg *pb.RequestMsg) (response *pb.ResponseMsg, err error) {
	log.Printf("[Node %d] Now Get start Phase1", node.label)
	*node.s1 = time.Now()
	node.SendMsgToNode()
	return &pb.ResponseMsg{}, nil
}

func (node *Node) Phase1ReceiveMsg(ctx context.Context, msg *pb.PointMsg) (response *pb.ResponseMsg, err error) {
	node.GetMsgFromNode(msg)
	return &pb.ResponseMsg{}, nil
}

func (node *Node) GetMsgFromNode(pointmsg *pb.PointMsg) (*pb.ResponseMsg, error) {
	*node.totMsgSize = *node.totMsgSize + proto.Size(pointmsg)
	index := pointmsg.GetIndex()
	log.Printf("Phase 1 :[Node %d] receive point message from [Node %d]", node.label, index)
	x := gmp.NewInt(0)
	x.SetBytes(pointmsg.GetX())
	y := gmp.NewInt(0)
	y.SetBytes(pointmsg.GetY())
	witness := node.dpc.NewG1()
	witness.SetCompressedBytes(pointmsg.Witness)
	p := point.Point{
		X:       x,
		Y:       y,
		PolyWit: witness,
	}
	//fmt.Println(p.X, node.label, p.Y, p.PolyWit)
	//Receive the point and store
	node.mutex.Lock()
	node.recPoint[*node.recCounter] = &p
	//fmt.Println(p)
	*node.recCounter += 1
	flag := (*node.recCounter == node.counter)
	node.mutex.Unlock()
	if flag {
		*node.recCounter = 0
		node.ClientReadPhase1()
	}
	return &pb.ResponseMsg{}, nil
}

//client
func (node *Node) SendMsgToNode() {
	if *node.iniflag {
		node.NodeConnect()
		*node.iniflag = false
	}
	p := point.Point{
		X:       node.secretShares[node.label-1].X,
		Y:       node.secretShares[node.label-1].Y,
		PolyWit: node.secretShares[node.label-1].PolyWit,
	}
	node.mutex.Lock()
	node.recPoint[*node.recCounter] = &p
	*node.recCounter += 1
	flag := *node.recCounter == node.counter
	node.mutex.Unlock()
	if flag {
		*node.recCounter = 0
		node.ClientReadPhase1()
	}
	var wg sync.WaitGroup
	for i := 0; i < node.counter; i++ {
		if i != node.label-1 {
			log.Printf("[Node %d] send point message to [Node %d]", node.label, i+1)
			//msg := point.Pointmsg{}
			//msg.SetIndex(node.label)
			//msg.SetPoint(node.secretShares[i])
			//(*node.Client[i]).GetMsgFromNode(msg)
			msg := &pb.PointMsg{
				Index:   int32(node.label),
				X:       node.secretShares[i].X.Bytes(),
				Y:       node.secretShares[i].Y.Bytes(),
				Witness: node.secretShares[i].PolyWit.CompressedBytes(),
				//Witness: nil,
			}
			//fmt.Println(node.secretShares[i].X, i+1, node.secretShares[i].Y, node.secretShares[i].PolyWit)
			wg.Add(1)
			go func(i int, msg *pb.PointMsg) {
				defer wg.Done()
				ctx, cancel := context.WithCancel(context.Background())
				_, err := node.nodeService[i].Phase1ReceiveMsg(ctx, msg)
				if err != nil {
					panic(err)
				}
				defer cancel()
			}(i, msg)
		}
	}
	wg.Wait()
}

//func (node *Node) Phase1() {
//	log.Printf("[Node %d] now start phase1", node.label)
//	x_point := make([]*gmp.Int, node.degree+1)
//	y_point := make([]*gmp.Int, node.degree+1)
//	for i := 0; i <= node.degree; i++ {
//		p := node.recPoint[i]
//		//x_point = append(x_point, gmp.NewInt(int64(point.X)))
//		x_point[i] = p.X
//		polyTmp.
//			y_point[i] = p.Y
//		//y_point = append(y_point, point.Y)
//	}
//	p, err := interpolation.LagrangeInterpolate(node.degree, x_point, y_point, node.p)
//	if err != nil {
//		for i := 0; i <= node.degree; i++ {
//			log.Print(x_point[i])
//			log.Print(y_point[i])
//		}
//		log.Print(err)
//		panic("Interpolation failed")
//	}
//	node.recPoly = &p
//	fmt.Printf("Interpolation finished\n")
//	//node.Phase2()
//}
// Read from the bulletinboard and does the interpolation and verifiication.
func (node *Node) ClientReadPhase1() {
	if *node.iniflag {
		node.NodeConnect()
		*node.iniflag = false
	}
	log.Printf("[node %d] read bulletinboard in phase 1", node.label)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := node.boardService.ReadPhase1(ctx, &pb.RequestMsg{})
	if err != nil {
		log.Fatalf("client failed to read phase1: %v", err)
	}
	for i := 0; i < node.counter; i++ {
		msg, err := stream.Recv()
		*node.totMsgSize = *node.totMsgSize + proto.Size(msg)
		if err != nil {
			log.Fatalf("client failed to receive in read phase1: %v", err)
		}
		index := msg.GetIndex()
		polycmt := msg.GetPolycmt()
		node.oldPolyCmt[index-1].SetCompressedBytes(polycmt)
	}
	x := make([]*gmp.Int, 0)
	y := make([]*gmp.Int, 0)
	polyCmt := node.dpc.NewG1()
	for i := 0; i <= node.degree; i++ {
		p := node.recPoint[i]
		x = append(x, p.X)
		y = append(y, p.Y)
		polyCmt.Set(node.oldPolyCmt[p.X.Int32()-1])
		if !node.dpc.VerifyEval(polyCmt, gmp.NewInt(int64(node.label)), p.Y, p.PolyWit) {
			//fmt.Println(node.label, p.X, "FAIL", polyCmt, p.Y, p.PolyWit)
			panic("Reconstruction Verification failed")
			//
		}

	}
	polyp, err := interpolation.LagrangeInterpolate(node.degree, x, y, node.p)
	if err != nil {
		for i := 0; i < len(x); i++ {
			log.Print(x[i])
			log.Print(y[i])
		}
		log.Print(err)
		panic("Interpolation failed")
	}
	node.recPoly.ResetTo(polyp)
	*node.e1 = time.Now()
	*node.s2 = time.Now()
	node.ClientSharePhase2()
}

//type ZeroMsg struct {
//	Index int32
//	Share []byte
//}
//
//func (msg *ZeroMsg) GetIndex() int32 {
//	return msg.Index
//}
//func (msg *ZeroMsg) GetShare() []byte {
//	return msg.Share
//}

//phase2
func (node *Node) ClientSharePhase2() {
	// Generate Random Numbers
	//fmt.Println("initial 0 is ",node._0Shares[node.counter-1])
	//for i := 0; i < node.counter; i++ {
	//	fmt.Println("lambda ",i+1 ,node.lambda[i])
	//}
	for i := 0; i < node.counter-1; i++ {
		node._0Shares[i].Rand(node.randState, gmp.NewInt(10))
		inter := gmp.NewInt(0)
		inter.Mul(node._0Shares[i], node.lambda[i])
		node._0Shares[node.counter-1].Sub(node._0Shares[node.counter-1], inter)
	}
	//0-share means the S
	node._0Shares[node.counter-1].Mod(node._0Shares[node.counter-1], node.p)
	inter := gmp.NewInt(0)
	inter.ModInverse(node.lambda[node.counter-1], node.p)
	node._0Shares[node.counter-1].Mul(node._0Shares[node.counter-1], inter)
	node._0Shares[node.counter-1].Mod(node._0Shares[node.counter-1], node.p)
	//////
	//exponentSum := node.dc.NewG1()
	//exponentSum.Set1()
	//Y := make([]*gmp.Int, node.counter)
	//Y[0] = gmp.NewInt(int64(16))
	//Y[1] = gmp.NewInt(int64(24))
	//Y[2] = gmp.NewInt(int64(30))
	//Y[3] = gmp.NewInt(int64(16))
	//Y[4] = gmp.NewInt(int64(0))
	//Y[4].SetString("57896044618658097711785492504343953926634992332820282019728792006155588075461",10)
	////fmt.Println("times ans iss ",exponentSum.Mul(exponentSum,node.dc.NewG1().PowBig(node.zerosumShareCmt[1],big.NewInt(0))))
	//for i := 0; i < node.counter; i++ {
	//	lambda := big.NewInt(0)
	//	lambda.SetString(node.lambda[i].String(), 10)
	//	//lambda.SetString(node.lambda[node.counter-1].String(), 10)
	//	tmp := node.dc.NewG1()
	//	node.dc.Commit(node.zeroShareCmt, Y[i])
	//	fmt.Println(i+1," 's cmt is ",node.zeroShareCmt)
	//	tmp.PowBig(node.zeroShareCmt, lambda)
	//	// log.Printf("label: %d #share %d\nlambda %s\nzeroshareCmt %s\ntmp %s", node.label, i+1, lambda.String(), node.zerosumShareCmt[i].String(), tmp.String())
	//	exponentSum.Mul(exponentSum, tmp)
	//	fmt.Println("exponentsun be ",exponentSum)
	//}
	//fmt.Println("should be 0 but be",exponentSum)
	//X:=make([]*gmp.Int,node.counter)
	//for i := 0; i < node.counter; i++ {
	//	X[i]=gmp.NewInt(int64(i+1))
	//}
	//Y:=make([]*gmp.Int,node.counter)
	//Y[0]=gmp.NewInt(int64(21))
	//Y[1]=gmp.NewInt(int64(22))
	//Y[2]=gmp.NewInt(int64(20))
	//Y[3]=gmp.NewInt(int64(20))
	//Y[4]=gmp.NewInt(int64(15))
	//tttmp ,_ :=interpolation.LagrangeInterpolate(node.counter-1,X,Y,node.p)
	//
	//fmt.Println("0 is ",tttmp.GetCoeffConstant())
	//for i := 0; i < node.counter; i++ {
	//	tmpppp :=gmp.NewInt(int64(0))
	//	tttmp.EvalMod(gmp.NewInt(int64(i+1)),node.p,tmpppp)
	//	fmt.Println(node.label,tmpppp.Mod(tmpppp.Sub(tmpppp,Y[i]),node.p))
	//}
	//to get sum for \sum_counter
	node.mutex.Lock()
	node._0ShareSum.Add(node._0ShareSum, node._0Shares[node.label-1])
	*node._0ShareCount = *node._0ShareCount + 1
	_0shareSumFinish := (*node._0ShareCount == node.counter)
	node.mutex.Unlock()

	if _0shareSumFinish {
		log.Printf("%d has finish _0ShareSum", node.label)
		*node._0ShareCount = 0
		//fmt.Println(node.label,"sum is  ",node._0ShareSum)
		node._0ShareSum.Mod(node._0ShareSum, node.p)
		//fmt.Println(node.label, "sum is  ", node._0ShareSum)

		//get a rand poly_tmp with 0-share
		//rand a poly_tmp polynomial
		node.dc.Commit(node.zeroShareCmt, node._0ShareSum)
		//fmt.Println(node.label," 's sum is ",node._0ShareSum," and the cmt is ",node.zeroShareCmt)
		polyTmp, _ := poly.NewRand(node.degree, node.randState, node.p)
		polyTmp.SetCoeffWithInt(0, 0)
		node.dpc.Commit(node.zeroPolyCmt, polyTmp)
		node.dpc.CreateWitness(node.zeroPolyWit, polyTmp, gmp.NewInt(0))

		err := polyTmp.SetCoeffWithGmp(0, node._0ShareSum)
		if err != nil {
			return
		}
		node.proPoly.ResetTo(polyTmp.DeepCopy())

		node.Phase2Write()
	}
	// share 0-share
	var wg sync.WaitGroup
	for i := 0; i < node.counter; i++ {
		if i != node.label-1 {
			log.Printf("[node %d] send message to [node %d] in phase 2", node.label, i+1)
			msg := &pb.ZeroMsg{
				Index: int32(node.label),
				Share: node._0Shares[i].Bytes(),
			}
			//fmt.Println(node.label,i+1,node._0Shares[i],"send")
			wg.Add(1)
			go func(i int, msg *pb.ZeroMsg) {
				defer wg.Done()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				_, err := node.nodeService[i].Phase2Share(ctx, msg)
				if err != nil {
					return
				}
			}(i, msg)
		}
	}
	wg.Wait()
}
func (node *Node) Phase2Share(ctx context.Context, msg *pb.ZeroMsg) (*pb.ResponseMsg, error) {
	*node.totMsgSize = *node.totMsgSize + proto.Size(msg)
	index := msg.GetIndex()
	log.Printf("[node %d] receive zero message from [node %d] in phase 2", node.label, index)
	inter := gmp.NewInt(0)
	inter.SetBytes(msg.GetShare())

	//fmt.Println(index,node.label,inter,"recive")
	//to get sum for \sum_counter
	node.mutex.Lock()
	node._0ShareSum.Add(node._0ShareSum, inter)
	*node._0ShareCount = *node._0ShareCount + 1
	_0shareSumFinish := (*node._0ShareCount == node.counter)
	node.mutex.Unlock()

	if _0shareSumFinish {
		log.Printf("%d has finish _0ShareSum", node.label)
		*node._0ShareCount = 0
		//fmt.Println(node.label,"sum is  ",node._0ShareSum)
		node._0ShareSum.Mod(node._0ShareSum, node.p)
		//fmt.Println(node.label, "sum is  ", node._0ShareSum)

		//get a rand poly_tmp with 0-share
		//rand a poly_tmp polynomial
		node.dc.Commit(node.zeroShareCmt, node._0ShareSum)
		//fmt.Println(node.label," 's sum is ",node._0ShareSum," and the cmt is ",node.zeroShareCmt)
		polyTmp, _ := poly.NewRand(node.degree, node.randState, node.p)
		polyTmp.SetCoeffWithInt(0, 0)
		node.dpc.Commit(node.zeroPolyCmt, polyTmp)
		node.dpc.CreateWitness(node.zeroPolyWit, polyTmp, gmp.NewInt(0))

		polyTmp.SetCoeffWithGmp(0, node._0ShareSum)
		node.proPoly.ResetTo(polyTmp.DeepCopy())

		node.Phase2Write()
	}
	return &pb.ResponseMsg{}, nil
}
func (node *Node) Phase2Write() {
	log.Printf("[node %d] write bulletinboard in phase 2", node.label)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Commitment and Witness from BulletinBoard
	msg := &pb.CommitMsg{
		Index:       int32(node.label),
		ShareCommit: node.zeroShareCmt.CompressedBytes(),
		PolyCommit:  node.zeroPolyCmt.CompressedBytes(),
		ZeroWitness: node.zeroPolyWit.CompressedBytes(),
	}
	//fmt.Println("t1 ",int32(node.label),node._0ShareSum,node.zeroShareCmt)
	node.boardService.WritePhase2(ctx, msg)
}
func (node *Node) Phase2Verify(ctx context.Context, request *pb.RequestMsg) (response *pb.ResponseMsg, err error) {
	log.Printf("[Node %d] start verification in phase 2", node.label)
	node.ClientReadPhase2()
	return &pb.ResponseMsg{}, nil
}
func (node *Node) ClientReadPhase2() {
	log.Printf("[node %d] read bulletinboard in phase 2", node.label)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := node.boardService.ReadPhase2(ctx, &pb.RequestMsg{})
	if err != nil {
		log.Fatalf("client failed to read phase2: %v", err)
	}
	for i := 0; i < node.counter; i++ {
		msg, err := stream.Recv()
		*node.totMsgSize = *node.totMsgSize + proto.Size(msg)
		if err != nil {
			log.Fatalf("client failed to receive in read phase1: %v", err)
		}
		index := msg.GetIndex()
		sharecmt := msg.GetShareCommit()
		polycmt := msg.GetPolyCommit()
		zerowitness := msg.GetZeroWitness()
		fmt.Println("t2 ", index, node.dpc.NewG1().SetCompressedBytes(sharecmt), node.dpc.NewG1().SetCompressedBytes(polycmt), node.dpc.NewG1().SetCompressedBytes(zerowitness), node.label)
		node.zerosumShareCmt[index-1].SetCompressedBytes(sharecmt)
		fmt.Println("t2 ", node.zerosumShareCmt[index-1], node.label)
		fmt.Println("hhhhhhhhhhhhhhhhh ", node.zerosumShareCmt[0], node.label)
		inter := node.dpc.NewG1()
		inter.SetString(node.zerosumShareCmt[index-1].String(), 10)
		node.zerosumPolyCmt[index-1].SetCompressedBytes(polycmt)
		node.midPolyCmt[index-1].Mul(inter, node.zerosumPolyCmt[index-1])
		node.zerosumPolyWit[index-1].SetCompressedBytes(zerowitness)
	}
	fmt.Println("hhhhhhhhhhhhhhhhh ", node.zerosumShareCmt[0], node.label)
	exponentSum := node.dc.NewG1()
	exponentSum.Set1()
	//fmt.Println("times ans iss ",exponentSum.Mul(exponentSum,node.dc.NewG1().PowBig(node.zerosumShareCmt[1],big.NewInt(0))))
	for i := 0; i < node.counter; i++ {
		lambda := big.NewInt(0)
		lambda.SetString(node.lambda[i].String(), 10)
		//lambda.SetString(node.lambda[node.counter-1].String(), 10)
		tmp := node.dc.NewG1()
		tmp.PowBig(node.zerosumShareCmt[i], lambda)
		// log.Printf("label: %d #share %d\nlambda %s\nzeroshareCmt %s\ntmp %s", node.label, i+1, lambda.String(), node.zerosumShareCmt[i].String(), tmp.String())
		exponentSum.Mul(exponentSum, tmp)
		fmt.Println(i+1, " 's cmt is", node.zerosumShareCmt[i], "Hey!!! exponentsun be ", exponentSum)
	}
	log.Printf("%d exponentSum: %s", node.label, exponentSum.String())
	//if !exponentSum.Is1() {
	//	panic("Proactivization Verification 1 failed")
	//}
	flag := true
	for i := 0; i < node.counter; i++ {
		if !node.dpc.VerifyEval(node.zerosumPolyCmt[i], gmp.NewInt(0), gmp.NewInt(0), node.zerosumPolyWit[i]) {
			flag = false
		}
	}
	if !flag {
		panic("Proactivization Verification 2 failed")
	}
	*node.e2 = time.Now()
	*node.s3 = time.Now()
	node.ClientSharePhase3()
}

//func Demo_test() {
//	var nodes [3]Node
//	var modp *gmp.Int
//	modp = GetPrime(256)
//	for i := 0; i < 3; i++ {
//		nodes[i], _ = New(1, i+1, 3, "/home/alan/Desktop", modp)
//	}
//	for i := 0; i < 3; i++ {
//		nodes[i].connect([]*Node{&nodes[0], &nodes[1], &nodes[2]})
//	}
//	for i := 0; i < 3; i++ {
//		nodes[i].SendMsgToNode()
//	}
//
//	for i := 0; i < 3; i++ {
//		nodes[i].ClientSharePhase2()
//	}
//
//	for i := 0; i < 3; i++ {
//		nodes[i].ClientSharePhase3()
//	}
//
//}

func (node *Node) NodeConnect() {
	boradConn, err := grpc.Dial(node.ipOfBoard, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Fail to connect board:%v", err)
	}
	node.boardConn = boradConn
	node.boardService = pb.NewBulletinBoardServiceClient(boradConn)
	for i := 0; i < node.counter; i++ {
		if i != node.label-1 {
			clientconn, err := grpc.Dial(node.ipAddress[i], grpc.WithInsecure())
			if err != nil {
				log.Fatalf("[Node %d] Fail to connect to other node:%v", node.label, err)
			}
			node.clientConn[i] = clientconn
			node.nodeService[i] = pb.NewNodeServiceClient(clientconn)
		}
	}
}
func (node *Node) Service() {
	port := node.ipAddress[node.label-1]
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("[Node %d] fail to listen:%v", node.label, err)
	}
	server := grpc.NewServer()
	pb.RegisterNodeServiceServer(server, node)
	reflection.Register(server)
	err = server.Serve(listener)
	if err != nil {
		log.Fatalf("[Node %d] fail to provide service", node.label)
	}

	log.Printf("[Node %d] now serve on %s", node.label, node.ipAddress[node.label-1])
}

func (node *Node) Serve(aws bool) {
	port := node.ipAddress[node.label-1]
	if aws {
		port = "0.0.0.0:12001"
	}
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("node failed to listen %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterNodeServiceServer(s, node)
	reflection.Register(s)
	log.Printf("node %d serve on %s", node.label, port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("node failed to serve %v", err)
	}
}
func ReadIpList(metadataPath string) []string {
	ipData, err := ioutil.ReadFile(metadataPath + "/ip_list")
	if err != nil {
		log.Fatalf("node failed to read iplist %v\n", err)
	}
	return strings.Split(string(ipData), "\n")
}

//第三部分的分享， 是全份额重建
//通过新的多项式B（x，i），对包括自己的每一个node发送B（i，i）
//最后重建多项式

//重建secretShare
func (node *Node) Phase3SendMsg(ctx context.Context, msg *pb.PointMsg) (*pb.ResponseMsg, error) {
	*node.totMsgSize = *node.totMsgSize + proto.Size(msg)
	index := msg.GetIndex()
	Y := msg.GetY()
	log.Printf("[node %d] receive point message from [node %d] in phase3", node.label, index)
	witness := msg.GetWitness()
	node.secretShares[index-1].Y.SetBytes(Y)
	node.secretShares[index-1].PolyWit.SetCompressedBytes(witness)
	node.mutex.Lock()
	*node.shareCnt = *node.shareCnt + 1
	flag := *node.shareCnt == node.counter
	node.mutex.Unlock()
	if flag {
		log.Printf("[node %d] has finish sharePhase3", node.label)
		*node.shareCnt = 0
		node.Phase3WriteOnBorad()
	}
	return &pb.ResponseMsg{}, nil
}
func (node *Node) ClientSharePhase3() {
	node.newPoly.Add(*node.recPoly, *node.proPoly)
	//fmt.Println(*node.recPoly)
	var wg sync.WaitGroup
	for i := 0; i < node.counter; i++ {
		value := gmp.NewInt(0)
		node.newPoly.EvalMod(gmp.NewInt(int64(i+1)), node.p, value)
		//witness
		witness := node.dpc.NewG1()
		node.dpc.CreateWitness(witness, *node.newPoly, gmp.NewInt(int64(i+1)))

		if i != node.label-1 {
			log.Printf("node %d send point message to node %d in phase 3", node.label, i+1)
			msg := &pb.PointMsg{
				Index:   int32(node.label),
				X:       gmp.NewInt(int64(node.label)).Bytes(),
				Y:       value.Bytes(),
				Witness: witness.CompressedBytes(),
			}
			//把消息发送给不同的节点
			wg.Add(1)
			go func(i int, msg *pb.PointMsg) {
				defer wg.Done()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				node.nodeService[i].Phase3SendMsg(ctx, msg)
			}(i, msg)
		} else {
			node.secretShares[i].Y.Set(value)
			node.secretShares[i].PolyWit.Set(witness)
			node.mutex.Lock()
			*node.shareCnt = *node.shareCnt + 1
			flag := *node.shareCnt == node.counter
			node.mutex.Unlock()
			if flag {
				*node.shareCnt = 0
				node.Phase3WriteOnBorad()
			}
		}
	}
}
func (node *Node) Phase3WriteOnBorad() {
	log.Printf("[node %d] write bulletinboard in phase 3", node.label)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	C := node.dpc.NewG1()
	node.dpc.Commit(C, *node.newPoly)
	msg := &pb.Cmt1Msg{
		Index:   int32(node.label),
		Polycmt: C.CompressedBytes(),
	}
	//fmt.Println(node.label,C)
	node.boardService.WritePhase3(ctx, msg)
	//log.Printf("finish~~")
}
func (node *Node) Phase3Verify(ctx context.Context, msg *pb.RequestMsg) (*pb.ResponseMsg, error) {
	log.Printf("[node %d] start verification in phase 3", node.label)
	node.Phase3Readboard()
	return &pb.ResponseMsg{}, nil
}

func (node *Node) Phase3Readboard() {
	log.Printf("[node %d] read bulletinboard in phase 3", node.label)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := node.boardService.ReadPhase3(ctx, &pb.RequestMsg{})
	if err != nil {
		log.Fatalf("client failed to read phase3: %v", err)
	}
	for i := 0; i < node.counter; i++ {
		msg, err := stream.Recv()
		*node.totMsgSize = *node.totMsgSize + proto.Size(msg)
		if err != nil {
			log.Fatalf("client failed to receive in read phase1: %v", err)
		}
		index := msg.GetIndex()
		polycmt := msg.GetPolycmt()
		node.newPolyCmt[index-1].SetCompressedBytes(polycmt)
	}
	//fmt.Println("hhhh")
	for i := 0; i < node.counter; i++ {
		tmp := node.dpc.NewG1()
		if !node.newPolyCmt[i].Equals(tmp.Mul(node.oldPolyCmt[i], node.midPolyCmt[i])) {
			panic("Share Distribution Verification 1 failed")
		}
		if !node.dpc.VerifyEval(node.newPolyCmt[i], gmp.NewInt(int64(node.label)), node.secretShares[i].Y, node.secretShares[i].PolyWit) {
			panic("Share Distribution Verification 2 failed")
		}

	}
	*node.e3 = time.Now()
	f, _ := os.OpenFile(node.metadataPath+"/log"+strconv.Itoa(node.label), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	//fmt.Fprintf(f, "totMsgSize,%d\n", *node.totMsgSize)
	//fmt.Fprintf(f, "epochLatency,%d\n", node.e3.Sub(*node.s1).Nanoseconds())
	//fmt.Fprintf(f, "reconstructionLatency,%d\n", node.e1.Sub(*node.s1).Nanoseconds())
	//fmt.Fprintf(f, "proactivizationLatency,%d\n", node.e2.Sub(*node.s2).Nanoseconds())
	//fmt.Fprintf(f, "sharedistLatency,%d\n", node.e3.Sub(*node.s3).Nanoseconds())
	*node.totMsgSize = 0
	for i := 0; i < node.counter; i++ {
		node._0Shares[i].SetInt64(0)
	}
	node._0ShareSum.SetInt64(0)
}

func New(degree int, label int, counter int, logPath string, coeff []*gmp.Int) (Node, error) {
	if label < 0 {
		return Node{}, errors.New("Label must be a non-negative number!")
	}
	file, _ := os.Create(logPath + "/log" + strconv.Itoa(label))
	defer file.Close()
	ipRaw := ReadIpList(logPath)[0 : counter+1]
	bip := ipRaw[0]
	ipList := ipRaw[1 : counter+1]
	if counter < 0 {
		return Node{}, errors.New("Counter must be a non-negative number!")
	}
	randState := rand.New(rand.NewSource(time.Now().Local().UnixNano()))
	//fixedRandState := rand.New(rand.NewSource(int64(3)))
	dc := commitment.DLCommit{}
	dc.SetupFix()
	dpc := commitment.DLPolyCommit{}
	dpc.SetupFix(counter)

	p := gmp.NewInt(0)
	p.SetString("57896044618658097711785492504343953926634992332820282019728792006155588075521", 10)
	lambda := make([]*gmp.Int, counter)
	// Calculate Lagrange Interpolation
	denominator := poly.NewConstant(1)
	tmp, _ := poly.NewPoly(1)
	tmp.SetCoeffWithInt(1, 1)
	for i := 0; i < counter; i++ {
		tmp.GetPtrtoConstant().Neg(gmp.NewInt(int64(i + 1)))
		denominator.MulSelf(tmp)
	}
	for i := 0; i < counter; i++ {
		lambda[i] = gmp.NewInt(0)
		deno, _ := poly.NewPoly(0)
		tmp.GetPtrtoConstant().Neg(gmp.NewInt(int64(i + 1)))

		deno.Divide(denominator, tmp)
		deno.EvalMod(gmp.NewInt(0), p, lambda[i])
		inter := gmp.NewInt(0)
		deno.EvalMod(gmp.NewInt(int64(i+1)), p, inter)
		interInv := gmp.NewInt(0)
		interInv.ModInverse(inter, p)
		lambda[i].Mul(lambda[i], interInv)
		lambda[i].Mod(lambda[i], p)
	}

	_0Shares := make([]*gmp.Int, counter)
	for i := 0; i < counter; i++ {
		_0Shares[i] = gmp.NewInt(0)
	}
	_0ShareCount := 0
	_0ShareSum := gmp.NewInt(0)

	recPoint := make([]*point.Point, counter)
	recCounter := 0

	secretShares := make([]*point.Point, counter)
	//tmpPoly, err := poly.NewRand(degree, fixedRandState, p)
	tmpPoly, err := poly.NewPoly(len(coeff) - 1)
	tmpPoly.SetbyCoeff(coeff)
	//fmt.Println(tmpPoly,coeff)

	for i := 0; i < counter; i++ {
		if err != nil {
			panic("Error initializing random tmpPoly")
		}
		x := int32(i + 1)
		y := gmp.NewInt(0)
		w := dpc.NewG1()
		tmpPoly.EvalMod(gmp.NewInt(int64(x)), p, y)
		dpc.CreateWitness(w, tmpPoly, gmp.NewInt(int64(x)))

		//fmt.Println(tmpPoly,x,"witness is ",w)
		secretShares[i] = point.NewPoint(gmp.NewInt(int64(label)), y, w)
		//fmt.Println(i+1,label,y,w)
	}

	proPoly, _ := poly.NewPoly(degree)
	recPoly, _ := poly.NewPoly(degree)
	newPoly, _ := poly.NewPoly(degree)
	shareCnt := 0

	oldPolyCmt := make([]*pbc.Element, counter)
	midPolyCmt := make([]*pbc.Element, counter)
	newPolyCmt := make([]*pbc.Element, counter)
	for i := 0; i < counter; i++ {
		oldPolyCmt[i] = dpc.NewG1()
		midPolyCmt[i] = dpc.NewG1()
		newPolyCmt[i] = dpc.NewG1()
	}

	zeroShareCmt := dc.NewG1()
	zeroPolyCmt := dpc.NewG1()
	zeroPolyWit := dpc.NewG1()

	zerosumShareCmt := make([]*pbc.Element, counter)
	zerosumPolyCmt := make([]*pbc.Element, counter)
	zerosumPolyWit := make([]*pbc.Element, counter)

	for i := 0; i < counter; i++ {
		zerosumShareCmt[i] = dc.NewG1()
		zerosumPolyCmt[i] = dpc.NewG1()
		zerosumPolyWit[i] = dpc.NewG1()
	}

	totMsgSize := 0
	s1 := time.Now()
	e1 := time.Now()
	s2 := time.Now()
	e2 := time.Now()
	s3 := time.Now()
	e3 := time.Now()

	clientConn := make([]*grpc.ClientConn, counter)
	nodeService := make([]pb.NodeServiceClient, counter)

	iniflag := true
	log.Printf("Node %d new done", label)
	return Node{
		metadataPath:    logPath,
		ipAddress:       ipList,
		ipOfBoard:       bip,
		degree:          degree,
		label:           label,
		counter:         counter,
		randState:       randState,
		dc:              &dc,
		dpc:             &dpc,
		p:               p,
		lambda:          lambda,
		_0Shares:        _0Shares,
		_0ShareCount:    &_0ShareCount,
		_0ShareSum:      _0ShareSum,
		secretShares:    secretShares,
		recPoint:        recPoint,
		recCounter:      &recCounter,
		recPoly:         &recPoly,
		proPoly:         &proPoly,
		newPoly:         &newPoly,
		shareCnt:        &shareCnt,
		oldPolyCmt:      oldPolyCmt,
		midPolyCmt:      midPolyCmt,
		newPolyCmt:      newPolyCmt,
		zeroShareCmt:    zeroShareCmt,
		zeroPolyCmt:     zeroPolyCmt,
		zeroPolyWit:     zeroPolyWit,
		zerosumPolyCmt:  zerosumShareCmt,
		zerosumPolyWit:  zerosumPolyWit,
		zerosumShareCmt: zerosumShareCmt,
		totMsgSize:      &totMsgSize,
		s1:              &s1,
		e1:              &e1,
		s2:              &s2,
		e2:              &e2,
		s3:              &s3,
		e3:              &e3,
		clientConn:      clientConn,
		nodeService:     nodeService,
		iniflag:         &iniflag,
	}, nil

}
