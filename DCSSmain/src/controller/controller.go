package controller

import (
	"context"
	"fmt"
	"github.com/Alan-Lxc/crypto_contest/dcssweb/common"
	"github.com/Alan-Lxc/crypto_contest/dcssweb/model"
	"github.com/Alan-Lxc/crypto_contest/src/basic/poly"
	"github.com/Alan-Lxc/crypto_contest/src/bulletboard"
	model1 "github.com/Alan-Lxc/crypto_contest/src/model"
	"github.com/Alan-Lxc/crypto_contest/src/nodes"
	pb "github.com/Alan-Lxc/crypto_contest/src/service"
	"github.com/ncw/gmp"
	"google.golang.org/grpc"
	"log"
	"math/rand"
	"time"
)

type Controll struct {
	//nodeConn
	nodeConn []*grpc.ClientConn
	//nodeservice
	nodeService []pb.NodeServiceClient
	//boardconn
	boardConn *grpc.ClientConn
	//boardService
	boardService pb.BulletinBoardServiceClient
	//metadatapath
	ipList          []string
	bulletboardList []string
	//bb num
	bbNum   int
	nodeNum int
}

// FORGOT TO KILL THR THREAD OF CONTROLL !!
var Controller *Controll

//这里写了metadatapath，后面就不需要写了
var metadatapath = "/home/kzl/Desktop/test/crypto_contest/DCSSmain/src/metadata"

func New() *Controll {
	return new(Controll)
}
func (controll *Controll) Initsystem(degree, counter int, metadatapath string, secretid int, polyyy []poly.Poly) {
	db := common.GetDB()
	if db != nil {

	}
	var nodeConnnect []*nodes.Node
	nConn := make([]*grpc.ClientConn, counter) //get from sql and new
	nodeService := make([]pb.NodeServiceClient, counter)
	ipList := nodes.ReadIpList(metadatapath + "/ip_list")
	for i := 0; i < counter; i++ {
		node, err := nodes.New_for_web(degree, i+1, counter, metadatapath)
		// here need to change merge NODE
		newunit := model.Unit{
			UnitId: node.GetLabel(),
			UnitIp: node.IpAddress[node.GetLabel()],
			//Secretnum: 0,
		}
		db.Create(&newunit)
		nodeConnnect = append(nodeConnnect, &node)
		if err != nil {
			println(err)
		}
		go node.Serve_for_web()
		Conn, err := grpc.Dial(ipList[i], grpc.WithInsecure())
		if err != nil {
			log.Fatalf("Fail to connect with %s:%v", ipList[i], err)
		}
		nConn[i] = Conn
		nodeService[i] = pb.NewNodeServiceClient(Conn)
	}
	bb, _ := bulletboard.New_bulletboard_for_web(degree, counter, metadatapath, secretid, polyyy)
	go bb.Serve(false)
	time.Sleep(2)
	bconn, _ := grpc.Dial(bb.Getbip(), grpc.WithInsecure())
	boardConn := bconn
	boardService := pb.NewBulletinBoardServiceClient(bconn)

	boradList := nodes.ReadIpList(metadatapath + "/bulletboard_list")
	//controll := new(Controll)
	controll.ipList = ipList
	controll.nodeConn = nConn
	controll.nodeService = nodeService
	controll.bulletboardList = boradList
	controll.boardConn = boardConn
	controll.boardService = boardService
	controll.bbNum = 1
	controll.nodeNum = counter
	//return controll
}
func (controll *Controll) GetMessageOfNode(secretid, label int) poly.Poly {

	db := common.GetDB()
	var secretshares []model1.Secretshare
	result := db.Where("secret_id =?", secretid).Where("unit_id", label).Find(&secretshares)
	rowNum := result.RowsAffected
	var newsecret model.Secret
	db.Where("id = ? ", secretid).First(&newsecret)
	degree := newsecret.Degree
	//counter := newsecretshare.Counter
	//secretid := int(newsecretshare.SecretId)
	coeff := make([]*gmp.Int, degree+1)
	for i := 0; int64(i) < rowNum; i++ {
		var newsecretshare model1.Secretshare
		db.Where("secret_id = ? and unit_id = ? and row_num =?", secretid, label, i).Find(&newsecretshare)
		//Data存放秘密份额,多项式
		Data := newsecretshare.Data

		coeff[i] = gmp.NewInt(0)
		coeff[i].SetBytes(Data)
	}
	tmpPoly, _ := poly.NewPoly(len(coeff) - 1)
	tmpPoly.SetbyCoeff(coeff)
	return tmpPoly
	//	return a poly
}
func (controll *Controll) Getmessage(secretid int, degree int, counter int) []poly.Poly {

	polyyy := make([]poly.Poly, counter)
	for i := 0; i < counter; i++ {
		polyyy[i] = controll.GetMessageOfNode(secretid, i+1)
	}
	return polyyy
}
func (controll *Controll) NewSecret(secretid int, degree int, counter int, s0 string) {

	fmt.Println(controll.bbNum)
	fixedRandState := rand.New(rand.NewSource(int64(3)))
	p := gmp.NewInt(0)
	p.SetString("57896044618658097711785492504343953926634992332820282019728792006155588075521", 10)
	tmp := gmp.NewInt(0)
	//tmp.SetString(s0, 10)
	tmp.SetString(s0, 10)
	polyy, _ := poly.NewRand(degree, fixedRandState, p)
	polyy.SetCoeffWithGmp(0, tmp)
	polyyy := make([]poly.Poly, counter)
	for i := 0; i < counter; i++ {
		x := int32(i + 1)
		y := gmp.NewInt(0)
		polyy.EvalMod(gmp.NewInt(int64(x)), p, y)

		polyyy[i], _ = poly.NewRand(degree*2, fixedRandState, p)
		polyyy[i].SetCoeffWithGmp(0, y)
	}
	controll.Initsystem(degree, counter, metadatapath, secretid, polyyy)

	var wg sync.WaitGroup
	for i := 0; i < counter; i++ {
		coeff := polyyy[i].GetAllCoeff()
		Coeff := make([][]byte, len(coeff))
		for j := 0; j < len(coeff); j++ {
			Coeff[j] = coeff[j].Bytes()
		}
		fmt.Println("coeff",coeff)
		fmt.Println("Coeff",Coeff)
		msg := pb.InitMsg{
			Degree:   int32(degree),
			Counter:  int32(counter),
			Secretid: int32(secretid),
			Coeff:    Coeff,
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			controll.nodeService[i].Initsecret(ctx, &msg)
		}(i)
	}
	wg.Wait()
}

func (controll *Controll) Handoff(secretid int, degree int, counter int) {

	log.Printf("Start to Handoff")
	polyyy := controll.Getmessage(secretid, degree, counter)
	//get degree, counter, metadatapath, secretid, polyyy
	controll.Initsystem(degree, counter, metadatapath, secretid, polyyy)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := controll.boardService.StartEpoch(ctx, &pb.RequestMsg{})
	if err != nil {
		log.Fatalf("Start Handoff Fail:%v", err)
	}
}

func (controll *Controll) Reconstruct(secretid int, degree int, counter int) string {
	log.Printf("Start to Reconstruction")

	//get degree, counter, metadatapath, secretid, polyyy
	//polyyy :=controll.Getmessage(secretid,degree,counter)
	//controll.Initsystem(degree, counter, metadatapath, secretid, polyyy)
	//ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()
	//_, err := controll.boardService.StartReconstruct(ctx, &pb.RequestMsg{})
	//if err != nil {
	//	log.Fatalf("Start Reconstruction Fail:%v", err)
	//}
	//// Set to string
	//return controll.boardService.gets0()
	return "123"
}

//package controller
//
//import (
//	"context"
//	"fmt"
//	"github.com/Alan-Lxc/crypto_contest/dcssweb/common"
//	"github.com/Alan-Lxc/crypto_contest/dcssweb/model"
//	"github.com/Alan-Lxc/crypto_contest/src/basic/poly"
//	"github.com/Alan-Lxc/crypto_contest/src/bulletboard"
//	"github.com/Alan-Lxc/crypto_contest/src/nodes"
//	pb "github.com/Alan-Lxc/crypto_contest/src/service"
//	"github.com/ncw/gmp"
//	"google.golang.org/grpc"
//	"log"
//	"math/rand"
//	"time"
//)
//
//type Controll struct {
//	//nodeConn
//	nodeConn []*grpc.ClientConn
//	//nodeservice
//	nodeService []pb.NodeServiceClient
//	//boardconn
//	boardConn []*grpc.ClientConn
//	//boardService
//	boardService []pb.BulletinBoardServiceClient
//	//metadatapath
//	ipList          []string
//	bulletboardList []string
//	//bb num
//	bbNum   int
//	nodeNum int
//}
//
//var Controller *Controll
//
////这里写了metadatapath，后面就不需要写了
//var metadatapath = "/home/kzl/Desktop/test/crypto_contest/DCSSmain/src/metadata"
//
//func Initsystem() *Controll {
//	db := common.GetDB()
//	if db != nil {
//
//	}
//	var nodeConnnect []*nodes.Node
//	nConn := make([]*grpc.ClientConn, 100) //get from sql and new
//	nodeService := make([]pb.NodeServiceClient, 100)
//	ipList := nodes.ReadIpList(metadatapath + "/ip_list")
//	for i := 0; i < 100; i++ {
//		node, err := nodes.New_for_web(i+1, metadatapath)
//		newunit := model.Unit{
//			UnitId:  node.GetLabel(),
//			UnitIp:  node.IpAddress[node.GetLabel()],
//			//Secretnum: 0,
//		}
//		db.Create(&newunit)
//		nodeConnnect = append(nodeConnnect, node)
//		if err != nil {
//			println(err)
//		}
//		go node.Serve_for_web()
//		Conn, err := grpc.Dial(ipList[i], grpc.WithInsecure())
//		if err != nil {
//			log.Fatalf("Fail to connect with %s:%v", ipList[i], err)
//		}
//		nConn[i] = Conn
//		nodeService[i] = pb.NewNodeServiceClient(Conn)
//	}
//	boradList := nodes.ReadIpList(metadatapath + "/bulletboard_list")
//	controll := new(Controll)
//	controll.ipList = ipList
//	controll.nodeConn = nConn
//	controll.nodeService = nodeService
//	controll.bulletboardList = boradList
//	controll.boardConn = make([]*grpc.ClientConn, 10)
//	controll.boardService = make([]pb.BulletinBoardServiceClient, 10)
//	controll.bbNum = 10
//	controll.nodeNum = 100
//	return controll
//}
//func (controll *Controll) NewSecret(secretid int, degree int, counter int, s0 string) {
//	fmt.Println(controll.bbNum)
//	fixedRandState := rand.New(rand.NewSource(int64(3)))
//	p := gmp.NewInt(0)
//	p.SetString("57896044618658097711785492504343953926634992332820282019728792006155588075521", 10)
//	tmp := gmp.NewInt(0)
//	//tmp.SetString(s0, 10)
//	tmp.SetString(s0, 10)
//	polyy, _ := poly.NewRand(degree, fixedRandState, p)
//	polyy.SetCoeffWithGmp(0, tmp)
//	polyyy := make([]poly.Poly, counter)
//	for i := 0; i < counter; i++ {
//		x := int32(i + 1)
//		y := gmp.NewInt(0)
//		polyy.EvalMod(gmp.NewInt(int64(x)), p, y)
//
//		polyyy[i], _ = poly.NewRand(degree*2, fixedRandState, p)
//		polyyy[i].SetCoeffWithGmp(0, y)
//	}
//	bb, _ := bulletboard.New_bulletboard_for_web(degree, counter, metadatapath, secretid, polyyy)
//	go bb.Serve(false)
//	time.Sleep(2)
//	bconn, err := grpc.Dial(bb.Getbip(), grpc.WithInsecure())
//	if err != nil {
//		log.Fatalf("System could not connect to bulletboard %d", secretid)
//	}
//	if secretid > controll.bbNum {
//		tmp1 := make([]*grpc.ClientConn, secretid-controll.bbNum)
//		controll.boardConn = append(controll.boardConn, tmp1...)
//		tmp2 := make([]pb.BulletinBoardServiceClient, secretid-controll.bbNum)
//		controll.boardService = append(controll.boardService, tmp2...)
//		controll.bbNum = secretid
//	}
//	controll.boardConn[secretid-1] = bconn
//	controll.boardService[secretid-1] = pb.NewBulletinBoardServiceClient(bconn)
//
//	if counter > controll.nodeNum {
//		tmp1 := make([]*grpc.ClientConn, counter-controll.nodeNum)
//		controll.nodeConn = append(controll.nodeConn, tmp1...)
//		tmp2 := make([]pb.NodeServiceClient, counter-controll.nodeNum)
//		controll.nodeService = append(controll.nodeService, tmp2...)
//		for i := controll.nodeNum; i < counter; i++ {
//			Conn, err := grpc.Dial(controll.ipList[i], grpc.WithInsecure())
//			if err != nil {
//				log.Fatalf("Fail to connect with %s:%v", controll.ipList[i], err)
//			}
//			controll.nodeConn[i] = Conn
//			controll.nodeService[i] = pb.NewNodeServiceClient(Conn)
//		}
//		controll.nodeNum = counter
//	}
//	for i := 0; i < counter; i++ {
//		coeff := polyyy[i].GetAllCoeff()
//		Coeff := make([][]byte, len(coeff))
//		for i := 0; i < len(coeff); i++ {
//// 			tmp := make([]byte, len(coeff[i].Bytes()))
//// 			tmp = coeff[i].Bytes()
//			Coeff[i] = coeff[i].Bytes()
//		}
//		msg := pb.InitMsg{
//			Degree:   int32(degree),
//			Counter:  int32(counter),
//			Secretid: int32(secretid),
//			Coeff:    Coeff,
//		}
//		ctx, cancel := context.WithCancel(context.Background())
//		defer cancel()
//		controll.nodeService[i].Initsecret(ctx, &msg)
//	}
//	//controll.Handoff(secretid)
//}
//
//func (controll *Controll) Handoff(secretid int) {
//	log.Printf("Start to Handoff")
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	_, err := controll.boardService[secretid-1].StartEpoch(ctx, &pb.RequestMsg{})
//	if err != nil {
//		log.Fatalf("Start Handoff Fail:%v", err)
//	}
//}
