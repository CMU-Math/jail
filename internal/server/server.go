package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/redpwn/jail/internal/config"
)

func RunProxy(cfg *config.Config) error {
	errCh := make(chan error)
	go runNsjailChild(errCh)
	go startProxy(cfg, errCh)
	return <-errCh
}

type program struct {
	stdin io.WriteCloser
	stdout bufio.Reader
	stderr io.ReadCloser
}

func makeProgram(cmd *exec.Cmd) program {
	ret := program{}
	ret.stdin, _ = cmd.StdinPipe()
	ret.stderr, _ = cmd.StderrPipe()

	stdout, _ := cmd.StdoutPipe()
	ret.stdout = *bufio.NewReader(stdout)

	return ret
}

const playerMntPath = "/mnt/players"
const graderMntPath = "/mnt/grader"

func RunDriver(cfg *config.Config) error {
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
			return err
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
			return err
		}

		rd := grader.stdout
		grader.stdin.Write([]byte(fmt.Sprintf("%d\n", numPlayers)))
		for {
			line, _, err := rd.ReadLine()
			if err != nil {
				return err
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

				return nil
			}
		}
	}

	return nil
}

func ExecServer(cfg *config.Config) error {
	if cfg.Pow > 0 {
		if err := execProxy(cfg); err != nil {
			return err
		}
	} else {
		if err := execNsjail(cfg); err != nil {
			return err
		}
	}
	return nil
}
