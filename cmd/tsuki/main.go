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

func FullOrRelative(filepath, wd string) string {
    if filepath[0] != '/' {
        filepath = path.Join(wd, filepath)
    }

    return filepath
}

func LocalWd() string {
    wd, err := os.Getwd()
    if err != nil {
        panic(err)
    }

    return wd
}

type NSClientConnector struct {
	NSAddr string
    chunkSize int
}

func UnmarshalNSResponse(response *http.Response) (msg *ClientMessage, err error) {
	buf := &bytes.Buffer{}
	io.Copy(buf, response.Body)

	msg = &ClientMessage{Status: response.StatusCode}

	if err = json.Unmarshal(buf.Bytes(), &msg); err != nil {
		log.Printf("warning: could not unmarshal response %q, %v", buf, err)
        err = fmt.Errorf("fundamental commucation protocol failure")
	}

	return
}

func (conn *NSClientConnector) GetNS(cmd, path string) (*ClientMessage, error) {
	addr := fmt.Sprintf("http://%s%s/%s?address=%s", conn.NSAddr, NSCLIENTPORT, cmd, path)

	resp, err := http.Get(addr)
	if err != nil {
		return nil, fmt.Errorf("request: %v", err)
	}

	msg, err := UnmarshalNSResponse(resp)
    if err != nil {
        return nil, fmt.Errorf("request: %v", err)
    }

	if msg.Status != http.StatusOK {
		return nil, fmt.Errorf(msg.Message)
	}

	return msg, nil
}

func (conn *NSClientConnector) GetNSInit() error {
	addr := fmt.Sprintf("http://%s%s/init", conn.NSAddr, NSCLIENTPORT)

	resp, err := http.Get(addr)
	if err != nil {
		return fmt.Errorf("request: %v", err)
	}

	msg, err := UnmarshalNSResponse(resp)
    if err != nil {
        return fmt.Errorf("request: %v", err)
    }

	if msg.Status != http.StatusOK {
		return fmt.Errorf(msg.Message)
	}

	return nil
}
func (conn *NSClientConnector) GetNSUpload(path string, size int64) (*ClientMessage, error) {
	addr := fmt.Sprintf("http://%s%s/upload?address=%s&size=%d", conn.NSAddr, NSCLIENTPORT, path, size)

	resp, err := http.Get(addr)
	if err != nil {
        return nil, fmt.Errorf("request: %v", err)
	}

	msg, err := UnmarshalNSResponse(resp)
    if err != nil {
        return nil, fmt.Errorf("request: %v", err)
    }

	if msg.Status != http.StatusOK {
		return nil, fmt.Errorf(msg.Message)
	}

	return msg, nil
}

func (conn *NSClientConnector) GetNSFromTo(cmd, from, to string) (*ClientMessage, error) {
	addr := fmt.Sprintf("http://%s%s/%s?from=%s&to=%s", conn.NSAddr, NSCLIENTPORT, cmd, from, to)

	resp, err := http.Get(addr)
	if err != nil {
        return nil, fmt.Errorf("request: %v", err)
	}

	msg, err := UnmarshalNSResponse(resp)
    if err != nil {
        return nil, fmt.Errorf("request: %v", err)
    }

	if msg.Status != http.StatusOK {
		return nil, fmt.Errorf(msg.Message)
	}

	return msg, nil
}

func (conn *NSClientConnector) GetNSObjectInfo(path string) (string, error) {
	addr := fmt.Sprintf("http://%s%s/info?address=%s", conn.NSAddr, NSCLIENTPORT, path)

	resp, err := http.Get(addr)
	if err != nil {
		return "", fmt.Errorf("request: %v", err)
	}

	msg, err := UnmarshalNSResponse(resp)
    if err != nil {
        return "", fmt.Errorf("request: %v", err)
    }

	if msg.Status != http.StatusOK {
		return "", fmt.Errorf(msg.Message)
	}

	return msg.Message, nil
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

func (conn *NSClientConnector) Cd(path string) error {
	msg, err := conn.GetNS("cd", path)
    if err != nil {
        return fmt.Errorf("cd: %v", err)
    }

	log.Printf("Received message: %#v", msg)

	return nil
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

func (conn *NSClientConnector) RemoveFile(path string) error {
	msg, err := conn.GetNS("rmfile", path)
	if err != nil {
		return fmt.Errorf("rm: %v", err)
	}

	log.Printf("Received message: %#v", msg)

	return nil
}

func (conn *NSClientConnector) MakeDir(path string) error {
	msg, err := conn.GetNS("mkdir", path)
	if err != nil {
		return fmt.Errorf("mkdir: %v", err)
	}

	log.Printf("Received message: %#v", msg)

	return nil
}

func (conn *NSClientConnector) RemoveDir(path string) error {
	msg, err := conn.GetNS("rmdir", path)
	if err != nil {
		return fmt.Errorf("rmdir: %v", err)
	}

	log.Printf("Received message: %#v", msg)

	return nil
}

func (conn *NSClientConnector) Copy(from, to string) error {
	msg, err := conn.GetNSFromTo("cp", from, to)
	if err != nil {
		return fmt.Errorf("cp: %v", err)
	}

	log.Printf("Received message: %#v", msg)

	return nil
}

func (conn *NSClientConnector) Move(from, to string) error {
	msg, err := conn.GetNSFromTo("mv", from, to)
	if err != nil {
		return fmt.Errorf("mv: %v", err)
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
        return fmt.Errorf("upload request: %v", err)
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

        err := conn.writeChunkToFS(meta.StorageIP, meta.ChunkID, msg.Token, barReader)
        if err != nil {
            return fmt.Errorf("upload sequence:")
        }

        uploaded += conn.chunkSize
        bar.Finish()
    }

	log.Printf("Received message: %#v", msg)

	return nil
}

func (conn *NSClientConnector) downloadChunk(addr, chunkId, token string, dest io.Writer) error {
    fsAddr := fmt.Sprintf("http://%s/chunks/%s?token=%s", addr, chunkId, token)
    resp, err := http.Get(fsAddr)
    if err != nil {
        return fmt.Errorf("fetch chunk: %v", err)
    }

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("fetch chunk: %d %s", resp.StatusCode, resp.Status)
    }

    _, err = io.Copy(dest, resp.Body)
    if err != nil {
        return fmt.Errorf("fetch chunk: %v", err)
    }

    return nil
}

func (conn *NSClientConnector) Download(srcPath string, file io.Writer) error {
    var err error
    if conn.chunkSize == 0 {
        conn.chunkSize, err = conn.GetChunkSize()
        if err != nil {
            return fmt.Errorf("download init: %v", err)
        }
    }

    msg, err := conn.GetNS("download", srcPath)
    if err != nil {
        return fmt.Errorf("download, request stage: %v", err)
    }

    for i, meta := range msg.Chunks {
        width := len(strconv.Itoa(len(msg.Chunks)))

        requestSize := conn.chunkSize

        bar := pb.ProgressBarTemplate(BarTemplate).Start(requestSize)
        bar.Set("chunkProgress", fmt.Sprintf("% *d/%d", width, i + 1, len(msg.Chunks)))

        barWriter := bar.NewProxyWriter(file)

        err := conn.downloadChunk(meta.StorageIP, meta.ChunkID, msg.Token, barWriter)
        if err != nil {
            return fmt.Errorf("download sequence: %v", err)
        }

        bar.Finish()
    }

	log.Printf("Received message: %#v", msg)

	return nil
}

func saveCwd() {
    filename := path.Join(os.TempDir(), TempCwd)
    file, err := os.Create(filename)
    if err != nil {
        log.Printf("warning: could not create temp file, %v", err)
        return
    }
    defer file.Close()

    fmt.Fprint(file, cwd)
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
                Name: "init",
                Usage: "Purge all data and initialize storage",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 0 {
                        return fmt.Errorf("error: provide no arguments")
                    }

                    err := conn.GetNSInit()
                    if err != nil {
                        return fmt.Errorf("error: %v", err)
                    }

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
                Name: "cd",
                Usage: "Move to directory",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 1 {
                        return fmt.Errorf("error: provide remote path")
                    }

                    newPath := FullOrRelative(c.Args().Get(0), cwd)

                    err := conn.Cd(newPath)
                    if err != nil {
                        return fmt.Errorf("error: %v", err)
                    }

                    cwd = newPath

                    saveCwd()

                    return nil
                },
            },
            {
                Name: "pwd",
                Usage: "Print current working directory",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 0 {
                        return fmt.Errorf("error: provide no arguments")
                    }

                    fmt.Println(cwd)

                    return nil
                },
            },
            {
                Name: "info",
                Usage: "Print REMOTE object info",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 1 {
                        return fmt.Errorf("error: provide a remote path to an object")
                    }

                    remotePath := FullOrRelative(c.Args().Get(0), cwd)

                    str, err := conn.GetNSObjectInfo(remotePath)
                    if err != nil {
                        return fmt.Errorf("error: %v", err)
                    }

                    fmt.Println(str)

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

                    dir := FullOrRelative(c.Args().Get(0), cwd)

                    err := conn.Touch(dir)
                    if err != nil {
                        return fmt.Errorf("error: %v", err)
                    }

                    return nil
                },
            },
            {
                Name: "mkdir",
                Usage: "Create empty directory",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 1 {
                        return fmt.Errorf("error: provide remote path to the directory")
                    }

                    remotePath := FullOrRelative(c.Args().Get(0), cwd)

                    err := conn.MakeDir(remotePath)
                    if err != nil {
                        return fmt.Errorf("error: %v", err)
                    }

                    return nil
                },
            },
            {
                Name: "upload",
                Usage: "Upload LOCAL file to REMOTE",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 2 {
                        return fmt.Errorf("error: provide local and remote paths to the file")
                    }

                    localPath  := FullOrRelative(c.Args().Get(0), LocalWd())
                    remotePath := FullOrRelative(c.Args().Get(1), cwd)

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
            {
                Name: "download",
                Usage: "Download REMOTE",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 2 {
                        return fmt.Errorf("error: provide remote and local paths to the file")
                    }

                    remotePath := FullOrRelative(c.Args().Get(0), cwd)
                    localPath  := FullOrRelative(c.Args().Get(1), LocalWd())

                    // TODO: do you want to overwrite?
                    if _, err := os.Stat(localPath); os.IsExist(err) {
                        log.Print("warning: overwrite")
                    }

                    file, err := os.Create(localPath)
                    if err != nil {
                        return fmt.Errorf("download: %v", err)
                    }
                    defer file.Close()

                    err = conn.Download(remotePath, file)
                    if err != nil {
                        return fmt.Errorf("error: %v", err)
                    }

                    return nil
                },
            },
            {
                Name: "mv",
                Usage: "Move REMOTE object to REMOTE",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 2 {
                        return fmt.Errorf("error: provide remote paths to the two objects")
                    }

                    srcPath  := FullOrRelative(c.Args().Get(0), cwd)
                    destPath := FullOrRelative(c.Args().Get(1), cwd)

                    err := conn.Move(srcPath, destPath)
                    if err != nil {
                        return fmt.Errorf("error: %v", err)
                    }

                    return nil
                },
            },
            {
                Name: "cp",
                Usage: "Copy REMOTE file to REMOTE",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 2 {
                        return fmt.Errorf("error: provide remote paths to the two files")
                    }

                    srcPath  := FullOrRelative(c.Args().Get(0), cwd)
                    destPath := FullOrRelative(c.Args().Get(1), cwd)

                    err := conn.Move(srcPath, destPath)
                    if err != nil {
                        return fmt.Errorf("error: %v", err)
                    }

                    return nil
                },
            },
            {
                Name: "rm",
                Usage: "Remove REMOTE file",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 1 {
                        return fmt.Errorf("error: provide remote path to the file")
                    }

                    remotePath := FullOrRelative(c.Args().Get(0), cwd)

                    err := conn.RemoveFile(remotePath)
                    if err != nil {
                        return fmt.Errorf("error: %v", err)
                    }

                    return nil
                },
            },
            {
                Name: "rmdir",
                Usage: "Remove REMOTE directory recursively",
                Action: func(c *cli.Context) error {
                    if c.Args().Len() != 1 {
                        return fmt.Errorf("error: provide remote path to the directory")
                    }

                    remotePath := FullOrRelative(c.Args().Get(0), cwd)

                    err := conn.RemoveDir(remotePath)
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
