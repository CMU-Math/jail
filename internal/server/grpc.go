package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"

	grader "github.com/CMU-Math/grader/internal/proto"
	"github.com/CMU-Math/grader/internal/cgroup"

	"golang.org/x/net/context"
  "google.golang.org/grpc"
  "google.golang.org/grpc/reflection"
)

const playerDirectory = "/mnt/players"

type graderServer struct {
	grader.UnimplementedGraderServiceServer
}

const playerMntPath = "/tmp/players"
const graderMntPath = "/tmp/grader"

func getPlayerPath(id string) string {
	return playerMntPath + "/" + id
}

func (g *graderServer) Grade(ctx context.Context, req *grader.Request) (*grader.Response, error) {
	os.MkdirAll(playerMntPath, 0755)

	for _, player := range req.GetPlayers() {
		err := os.WriteFile(getPlayerPath(player.GetId()), player.GetExecutable().GetCode(), 0755)
		if err != nil {
			return nil, err
		}
	}

	if err := os.WriteFile(graderMntPath, req.GetGrader().GetCode(), 0755); err != nil {
		return nil, err
	}

	var ret []string
	for i := uint32(0); i < req.GetIters(); i++ {
		res, err := runDriver()
		if err != nil {
			return nil, err
		}

		if err := cgroup.CleanupV1(); err != nil {
			return nil, err
		}

		ret = append(ret, res)
	}

	return &grader.Response{
		Response: ret,
	}, nil
}

type program struct {
	stdin io.WriteCloser
	stdout bufio.Reader
	stderr io.ReadCloser
	cmd *exec.Cmd
}

func makeProgram(cmd *exec.Cmd) program {
	ret := program{}
	ret.stdin, _ = cmd.StdinPipe()
	ret.stderr, _ = cmd.StderrPipe()

	stdout, _ := cmd.StdoutPipe()
	ret.stdout = *bufio.NewReader(stdout)

	ret.cmd = cmd

	return ret
}

func runDriver() (string, error) {
	playerFiles, _ := ioutil.ReadDir(playerMntPath)

	numPlayers := len(playerFiles)

	programs := make([]program, len(playerFiles))
	for i := 0; i < numPlayers; i++ {
		cmd := exec.Command("/jail/run")
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("JAIL_ID=%d", i),
			"JAIL_BINARY_PATH=" + playerMntPath + "/" + playerFiles[i].Name(),
		)

		programs[i] = makeProgram(cmd)

		if err := cmd.Start(); err != nil {
			return "", err
		}
	}

	{
		cmd := exec.Command("/jail/run")
		cmd.Env = append(os.Environ(),
			"JAIL_ID=grader",
			"JAIL_BINARY_PATH=" + graderMntPath,
		)

		grader := makeProgram(cmd)

		if err := cmd.Start(); err != nil {
			return "", err
		}

		rd := grader.stdout
		grader.stdin.Write([]byte(fmt.Sprintf("%d\n", numPlayers)))
		for {
			line, _, err := rd.ReadLine()
			if err != nil {
				return "", err
			}

			var result map[string]interface{}
			json.Unmarshal(line, &result)

			switch (result["kind"].(string)) {
			case "write":
				idx := int(result["player"].(float64))
				bytes, _ := json.Marshal(result["data"])
				programs[idx].stdin.Write(bytes)
				programs[idx].stdin.Write([]byte("\n"))
			case "read":
				idx := int(result["player"].(float64))
				line, _, err := programs[idx].stdout.ReadLine()
				if err != nil {
					cmdBytes, _ := io.ReadAll(programs[idx].stderr)
					fmt.Println(idx, "errored")
					fmt.Println(string(cmdBytes))

					grader.stdin.Write([]byte("null\n"))
				} else {
					grader.stdin.Write(line)
					grader.stdin.Write([]byte("\n"))
				}
			case "status":
				fmt.Println("status:")
				fmt.Println(result["data"].(string))
			case "end":
				fmt.Println("end")
				bytes, _ := json.Marshal(result["data"])
				fmt.Println(string(bytes))

				for _, player := range programs {
					if err := player.cmd.Process.Kill(); err != nil {
						return "", err
					}

					if _, err := player.cmd.Process.Wait(); err != nil {
						return "", err
					}
				}
				if err := grader.cmd.Process.Kill(); err != nil {
					return "", err
				}
				if _, err := grader.cmd.Process.Wait(); err != nil {
					return "", err
				}

				return string(bytes), nil
			}
		}
	}
}

func RunGRPC() error {
	fmt.Println("starting server")
	port := 8080
  lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
  if err != nil {
		return err
  }

  graderServer := &graderServer{}

  // The grpcServer is currently configured to serve h2c traffic by default.
  // To configure credentials or encryption, see: https://grpc.io/docs/guides/auth.html#go
  grpcServer := grpc.NewServer()
  reflection.Register(grpcServer)
  grader.RegisterGraderServiceServer(grpcServer, graderServer)
  grpcServer.Serve(lis)

	return nil
}
