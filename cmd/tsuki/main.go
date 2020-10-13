package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/cheggaaa/pb/v3"
	"github.com/urfave/cli/v2"
)

const (
    TempNS = "tsuki.ns"
    TempCwd = "tsuki.cwd"
)

const NSCLIENTPORT = ":7070"

const BarTemplate = ` chunk {{ string . "chunkProgress" }}   {{ percent . }} {{ speed . }}`

var ns string
var cwd string

type ChunkMessage struct {
	ChunkID   string `json:"chunkID"`
	StorageIP string `json:"storageIP"`
}

type ClientMessage struct {
	Status  int            `json:"-"`
	Message string         `json:"message"`
	Objects []string       `json:"objects"`
	Token   string         `json:"token"`
	Chunks  []ChunkMessage `json:"chunks"`
}

type NSClientConnector struct {
	NSAddr string
    chunkSize int
}

func UnmarshalNSResponse(response *http.Response) (msg *ClientMessage) {
	buf := &bytes.Buffer{}
	io.Copy(buf, response.Body)

	msg = &ClientMessage{Status: response.StatusCode}

	if err := json.Unmarshal(buf.Bytes(), &msg); err != nil {
		log.Printf("warning: could not unmarshal response %q, %v", buf, err)
	}

	return
}

func (conn *NSClientConnector) GetNS(cmd, path string) (*ClientMessage, error) {
	addr := fmt.Sprintf("http://%s%s/%s?address=%s", conn.NSAddr, NSCLIENTPORT, cmd, path)

	resp, err := http.Get(addr)
	if err != nil {
		return nil, fmt.Errorf("send request: %v", err)
	}

	msg := UnmarshalNSResponse(resp)

	if msg.Status != http.StatusOK {
		return nil, fmt.Errorf(msg.Message)
	}

	return msg, nil
}

func (conn *NSClientConnector) GetNSUpload(path string, size int64) (*ClientMessage, error) {
	addr := fmt.Sprintf("http://%s%s/upload?address=%s&size=%d", conn.NSAddr, NSCLIENTPORT, path, size)

	resp, err := http.Get(addr)
	if err != nil {
        return nil, fmt.Errorf("send request: %v", err)
	}

	msg := UnmarshalNSResponse(resp)

	if msg.Status != http.StatusOK {
		return nil, fmt.Errorf(msg.Message)
	}

	return msg, nil
}

func (conn *NSClientConnector) GetChunkSize() (int, error) {
	addr := fmt.Sprintf("http://%s%s/getChunkSize", conn.NSAddr, NSCLIENTPORT)

	resp, err := http.Get(addr)
	if err != nil {
		return 0, fmt.Errorf("send chunk size request: %v", err)
	}

    buf := &bytes.Buffer{}
    io.Copy(buf, resp.Body)

    var size int
    if err := json.Unmarshal(buf.Bytes(), &size); err != nil {
        return 0, fmt.Errorf("unmarshal chunk size response: %v\nbody: %q", err, buf)
    }

    log.Println(buf)

	return size, nil
}

func (conn *NSClientConnector) Ls(path string) ([]string, error) {
	msg, err := conn.GetNS("ls", path)
    if err != nil {
        return nil, fmt.Errorf("ls: %v", err)
    }

	log.Printf("Received message: %#v", msg)

	return msg.Objects, nil
}

func (conn *NSClientConnector) Touch(path string) error {
	msg, err := conn.GetNS("touch", path)
	if err != nil {
		return fmt.Errorf("touch: %v", err)
	}

	log.Printf("Received message: %#v", msg)

	return nil
}

func (conn *NSClientConnector) writeChunkToFS(addr, chunkId, token string, src io.Reader) error {
    fsAddr := fmt.Sprintf("http://%s/chunks/%s?token=%s", addr, chunkId, token)
    resp, err := http.Post(fsAddr, "application/octet-stream", src)
    if err != nil {
        return fmt.Errorf("send chunk: %v", err)
    }

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("send chunk: %d %s", resp.StatusCode, resp.Status)
    }

    return nil
}

func (conn *NSClientConnector) Upload(file io.Reader, destPath string, fileSize int64) error {
    var err error
    if conn.chunkSize == 0 {
        conn.chunkSize, err = conn.GetChunkSize()
        if err != nil {
            return fmt.Errorf("upload init: %v", err)
        }
    }

    msg, err := conn.GetNSUpload(destPath, fileSize)
    if err != nil {
        return fmt.Errorf("upload: %v", err)
    }

    uploaded := 0
    for i, meta := range msg.Chunks {
        width := len(strconv.Itoa(len(msg.Chunks)))

        requestSize := conn.chunkSize
        if int(fileSize) - uploaded < requestSize {
            requestSize = int(fileSize) - uploaded
        }

        bar := pb.ProgressBarTemplate(BarTemplate).Start(requestSize)
        bar.Set("chunkProgress", fmt.Sprintf("% *d/%d", width, i + 1, len(msg.Chunks)))

        chunkSrc := io.LimitReader(file, int64(conn.chunkSize))
        barReader := bar.NewProxyReader(chunkSrc)

        conn.writeChunkToFS(meta.StorageIP, meta.ChunkID, msg.Token, barReader)

        uploaded += conn.chunkSize
        bar.Finish()
    }

	log.Printf("Received message: %#v", msg)

	return nil
}

func saveCwd() {
}

func loadFromTemp(name string) string {
    filename := path.Join(os.TempDir(), name)
    if _, err := os.Stat(filename); os.IsNotExist(err) {
        return ""
    }

    file, err := os.Open(filename)
    if err != nil {
        log.Printf("warning: could not open temp file, %v", err)
        return ""
    }
    defer file.Close()

    buf := &bytes.Buffer{}
    io.Copy(buf, file)

    return buf.String()
}

func main() {

    ns := loadFromTemp(TempNS)

	conn := &NSClientConnector{
		NSAddr: ns,
	}
    log.Print(ns)

    cwd = loadFromTemp(TempCwd)
    if cwd == "" {
        cwd = "/"
        saveCwd()
    }

	app := &cli.App{
		Name:  "tsuki",
		Usage: "a CLI interface to tsukiFS distributed file system",
        Commands: []*cli.Command {
            {
                Name: "connect",
                Usage: "Probe and remember name server for future calls",
                Action: func(c *cli.Context) error {
                    conn.NSAddr = c.Args().First()

                    _, err := conn.Ls("/")
                    if err != nil {
                        return fmt.Errorf("%v", err)
                    }

                    file, err := os.Create(path.Join(os.TempDir(), TempNS))
                    if err != nil {
                        fmt.Printf("warning: could not create NS temp file: %v", err)
                        return nil
                    }
                    defer file.Close()

                    fmt.Fprint(file, conn.NSAddr)

                    return nil
                },
            },
            {
                Name: "ls",
                Usage: "List directory",
                Action: func(c *cli.Context) error {

                    dir := c.Args().First()
                    if dir == "" {
                        dir = cwd
                    }

                    objects, err := conn.Ls(dir)
                    if err != nil {
                        return fmt.Errorf("error: %v", err)
                    }

                    for _, obj := range objects {
                        fmt.Println(obj)
                    }

                    return nil
                },
            },
            {
                Name: "touch",
                Usage: "Create empty file",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 1 {
                        return fmt.Errorf("error: only provide path to the file")
                    }

                    dir := c.Args().First()
                    if dir[0] != '/' {
                        dir = path.Join(cwd, dir)
                    }

                    err := conn.Touch(dir)
                    if err != nil {
                        return fmt.Errorf("error: %v", err)
                    }

                    return nil
                },
            },
            {
                Name: "upload",
                Usage: "Upload file from local storage",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 2 {
                        return fmt.Errorf("error: provide local and remote paths to the file")
                    }

                    localWd, err := os.Getwd()
                    if err != nil {
                        panic(err)
                    }

                    localPath := c.Args().Get(0)
                    if localPath[0] != '/' {
                        localPath = path.Join(localWd, localPath)
                    }

                    remotePath := c.Args().Get(1)
                    if remotePath[0] != '/' {
                        remotePath = path.Join(cwd, remotePath)
                    }

                    file, err := os.Open(localPath)
                    if err != nil {
                        return fmt.Errorf("upload: %v", err)
                    }
                    defer file.Close()

                    stat, err := file.Stat()
                    if err != nil {
                        return fmt.Errorf("upload: %v", err)
                    }

                    err = conn.Upload(file, remotePath, stat.Size())
                    if err != nil {
                        return fmt.Errorf("error: %v", err)
                    }

                    return nil
                },
            },
        },
	}

    err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}
