package skilltool

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

var binaryCache sync.Map

type ExecuteRequest struct {
	Lang  string   `json:"lang"`
	Src   string   `json:"src"`
	Stdin string   `json:"stdin,omitempty"`
	Args  []string `json:"args,omitempty"`
}

type ExecuteResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Status int    `json:"status"`
	Error  string `json:"error,omitempty"`
}

type Executor struct {
	endpoint string
	client   *http.Client
}

func NewExecutor(endpoint string) *Executor {
	return &Executor{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 60 * time.Second},
	}
}

type judgeFile struct {
	Content *string `json:"content,omitempty"`
	FileID  *string `json:"fileId,omitempty"`
	Name    *string `json:"name,omitempty"`
	Max     *int64  `json:"max,omitempty"`
}

type judgeCmd struct {
	Args          []string              `json:"args"`
	Env           []string              `json:"env"`
	Files         []judgeFile           `json:"files"`
	CPULimit      uint64                `json:"cpuLimit"`
	MemoryLimit   uint64                `json:"memoryLimit"`
	ProcLimit     uint64                `json:"procLimit"`
	CopyIn        map[string]judgeFile  `json:"copyIn,omitempty"`
	CopyOut       []string              `json:"copyOut,omitempty"`
	CopyOutCached []string              `json:"copyOutCached,omitempty"`
}

type judgeRequest struct {
	Cmd []judgeCmd `json:"cmd"`
}

type judgeResult struct {
	Status     string            `json:"status"`
	ExitStatus int               `json:"exitStatus"`
	Error      string            `json:"error,omitempty"`
	Files      map[string]string `json:"files,omitempty"`
	FileIds    map[string]string `json:"fileIds,omitempty"`
}

type judgeResponse []judgeResult

type langConfig struct {
	srcName     string
	compileArgs []string
	runArgs     []string
	needCompile bool
	binaryName  string
}

var supportedLangs = map[string]langConfig{
	"python3": {
		srcName: "script.py",
		runArgs: []string{"python3", "script.py"},
	},
	"javascript": {
		srcName: "script.js",
		runArgs: []string{"node", "script.js"},
	},
	"go": {
		srcName:     "main.go",
		compileArgs: []string{"go", "build", "-o", "prog", "main.go"},
		runArgs:     []string{"./prog"},
		needCompile: true,
		binaryName:  "prog",
	},
	"c": {
		srcName:     "prog.c",
		compileArgs: []string{"gcc", "-O2", "-Wall", "prog.c", "-o", "prog"},
		runArgs:     []string{"./prog"},
		needCompile: true,
		binaryName:  "prog",
	},
	"cpp": {
		srcName:     "prog.cpp",
		compileArgs: []string{"g++", "-O2", "-Wall", "prog.cpp", "-o", "prog"},
		runArgs:     []string{"./prog"},
		needCompile: true,
		binaryName:  "prog",
	},
	"java": {
		srcName:     "Main.java",
		compileArgs: []string{"javac", "Main.java"},
		runArgs:     []string{"java", "Main"},
		needCompile: true,
	},
}

func strPtr(s string) *string { return &s }
func int64Ptr(v int64) *int64 { return &v }

func srcHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:16])
}

func buildFiles(stdin string) []judgeFile {
	maxStdout := int64(10485760)
	nameStdout := "stdout"
	nameStderr := "stderr"

	return []judgeFile{
		{Content: strPtr(stdin)},
		{Name: &nameStdout, Max: &maxStdout},
		{Name: &nameStderr, Max: &maxStdout},
	}
}

func (e *Executor) sendRequest(cmds []judgeCmd) (judgeResponse, error) {
	judgeReq := judgeRequest{Cmd: cmds}
	body, err := json.Marshal(judgeReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", e.endpoint+"/run", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("go-judge error [%d]: %s", resp.StatusCode, string(respBody))
	}

	var judgeResp judgeResponse
	if err := json.Unmarshal(respBody, &judgeResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(judgeResp) == 0 {
		return nil, fmt.Errorf("empty response from go-judge")
	}

	return judgeResp, nil
}

func (e *Executor) Execute(req *ExecuteRequest) (*ExecuteResponse, error) {
	cfg, ok := supportedLangs[req.Lang]
	if !ok {
		return nil, fmt.Errorf("不支持的语言: %s", req.Lang)
	}

	env := []string{
		"PATH=/usr/bin:/usr/local/bin:/usr/lib/jvm/java-11-openjdk/bin:/usr/local/go/bin",
		"GOCACHE=/tmp/gocache",
		"HOME=/tmp",
	}

	common := judgeCmd{
		Env:         env,
		Files:       buildFiles(req.Stdin),
		CPULimit:    30000000000,
		MemoryLimit: 536870912,
		ProcLimit:   300,
	}

	buildExecResp := func(r judgeResult) *ExecuteResponse {
		out := &ExecuteResponse{
			Status: r.ExitStatus,
			Error:  r.Error,
		}
		if r.Files != nil {
			out.Stdout = r.Files["stdout"]
			out.Stderr = r.Files["stderr"]
		}
		return out
	}

	if cfg.needCompile && cfg.binaryName != "" {
		hash := srcHash(req.Src)

		if cachedFileID, ok := binaryCache.Load(hash); ok {
			fileID := cachedFileID.(string)
			log.Printf("[go-judge] cache hit lang=%s hash=%s", req.Lang, hash[:12])

			runCmd := common
			runCmd.Args = append(cfg.runArgs, req.Args...)
			runCmd.CopyIn = map[string]judgeFile{
				cfg.binaryName: {FileID: &fileID},
			}

			runResp, err := e.sendRequest([]judgeCmd{runCmd})
			if err != nil {
				return nil, err
			}

			return buildExecResp(runResp[0]), nil
		}

		compileCmd := common
		compileCmd.Args = cfg.compileArgs
		compileCmd.CopyIn = map[string]judgeFile{
			cfg.srcName: {Content: strPtr(req.Src)},
		}
		compileCmd.CopyOutCached = []string{cfg.binaryName}

		compileResp, err := e.sendRequest([]judgeCmd{compileCmd})
		if err != nil {
			return nil, err
		}

		compileResult := compileResp[0]
		log.Printf("[go-judge] compile exitStatus=%d fileIds=%v error=%s", compileResult.ExitStatus, compileResult.FileIds, compileResult.Error)

		if compileResult.ExitStatus != 0 && compileResult.ExitStatus != -1 {
			return buildExecResp(compileResult), nil
		}

		fileID := ""
		if compileResult.FileIds != nil {
			fileID = compileResult.FileIds[cfg.binaryName]
		}
		if fileID == "" {
			log.Printf("[go-judge] fileID empty for %s, skipping run step", cfg.binaryName)
			return buildExecResp(compileResult), nil
		}

		binaryCache.Store(hash, fileID)

		runCmd := common
		runCmd.Args = append(cfg.runArgs, req.Args...)
		runCmd.CopyIn = map[string]judgeFile{
			cfg.binaryName: {FileID: &fileID},
		}

		runResp, err := e.sendRequest([]judgeCmd{runCmd})
		if err != nil {
			return nil, err
		}

		runResult := runResp[0]
		log.Printf("[go-judge] run exitStatus=%d files=%v error=%s", runResult.ExitStatus, runResult.Files, runResult.Error)

		respOut := buildExecResp(runResult)
		if respOut.Stderr == "" && compileResult.Files != nil {
			respOut.Stderr = compileResult.Files["stderr"]
		}
		return respOut, nil
	}

	if cfg.needCompile {
		// Compiled languages without binaryName (e.g., Java):
		// Keep old behavior — send both commands in one request
		compileCmd := common
		compileCmd.Args = cfg.compileArgs
		compileCmd.CopyIn = map[string]judgeFile{
			cfg.srcName: {Content: strPtr(req.Src)},
		}

		runCmd := common
		runCmd.Args = append(cfg.runArgs, req.Args...)

		judgeResp, err := e.sendRequest([]judgeCmd{compileCmd, runCmd})
		if err != nil {
			return nil, err
		}

		if len(judgeResp) >= 2 {
			compileResult := judgeResp[0]
			runResult := judgeResp[1]

			if compileResult.ExitStatus != 0 && compileResult.ExitStatus != -1 {
				return buildExecResp(compileResult), nil
			}

			respOut := buildExecResp(runResult)
			if respOut.Stderr == "" && compileResult.Files != nil {
				respOut.Stderr = compileResult.Files["stderr"]
			}
			return respOut, nil
		}

		return buildExecResp(judgeResp[len(judgeResp)-1]), nil
	}

	// Interpreted languages: single request
	runCmd := common
	runCmd.Args = append(cfg.runArgs, req.Args...)
	runCmd.CopyIn = map[string]judgeFile{
		cfg.srcName: {Content: strPtr(req.Src)},
	}

	judgeResp, err := e.sendRequest([]judgeCmd{runCmd})
	if err != nil {
		return nil, err
	}

	return buildExecResp(judgeResp[len(judgeResp)-1]), nil
}

func (e *Executor) ExecuteWithStdin(lang, src, stdin string) (*ExecuteResponse, error) {
	return e.Execute(&ExecuteRequest{
		Lang:  lang,
		Src:   src,
		Stdin: stdin,
	})
}

func SupportedLanguages() []string {
	langs := make([]string, 0, len(supportedLangs))
	for lang := range supportedLangs {
		langs = append(langs, lang)
	}
	return langs
}
