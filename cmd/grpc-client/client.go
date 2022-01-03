package main

import (
	"crypto/tls"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"log"
	"strconv"
	"time"

	pb "github.com/CMU-Math/grader/internal/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const REQUIRED = "REQIURED"

var (
	serverAddr         = flag.String("server_addr", "127.0.0.1:8080", "The server address in the format of host:port")
	serverHostOverride = flag.String("server_host_override", "", "")
	insecure           = flag.Bool("insecure", false, "Set to true to skip SSL validation")
	skipVerify         = flag.Bool("skip_verify", false, "Set to true to skip server hostname verification in SSL validation")
	playersPath        = flag.String("players", REQUIRED, "Directory with player files")
	graderPath         = flag.String("grader", REQUIRED, "Path to grader file")
	iters              = flag.Int("iters", 1, "Iterations to grade")
)

func main() {
	flag.Parse()

	var opts []grpc.DialOption
	if *serverHostOverride != "" {
		opts = append(opts, grpc.WithAuthority(*serverHostOverride))
	}
	if *insecure {
		opts = append(opts, grpc.WithInsecure())
	} else {
		cred := credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: *skipVerify,
		})
		opts = append(opts, grpc.WithTransportCredentials(cred))
	}
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewGraderServiceClient(conn)
	if err := grade(client); err != nil {
		log.Fatalf("grader failed: %v", err)
	}
}

func grade(client pb.GraderServiceClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	playerFiles, err := ioutil.ReadDir(*playersPath)
	if err != nil {
		return err
	}

	grader, err := os.ReadFile(*graderPath)

	var players []*pb.Player
	for idx, playerFile := range playerFiles {
		player, err := os.ReadFile(filepath.Join(*playersPath, playerFile.Name()))
		if err != nil {
			return err
		}
		players = append(players, &pb.Player{
			Executable: &pb.Executable{ Code: player },
			Id: strconv.Itoa(idx),
		})
	}

	rep, err := client.Grade(ctx, &pb.Request{
		Iters: uint32(*iters),
		Players: players,
		Grader: &pb.Executable{ Code: grader },
	})
	if err != nil {
		log.Fatalf("%v.Ping failed %v: ", client, err)
	}
	log.Printf("Ping got %v\n", rep.GetResponse())

	return nil
}

