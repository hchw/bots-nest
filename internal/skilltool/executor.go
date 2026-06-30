package skilltool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ExecuteRequest struct {
	Lang  string `json:"lang"`
	Src   string `json:"src"`
	Stdin string `json:"stdin,omitempty"`
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
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

type judgeFile struct {
	Content *string `json:"content,omitempty"`
	Name    *string `json:"name,omitempty"`
	Max     *int64  `json:"max,omitempty"`
}

type judgeCmd struct {
	Args        []string              `json:"args"`
	Env         []string              `json:"env"`
	Files       []judgeFile           `json:"files"`
	CPULimit    uint64                `json:"cpuLimit"`
	MemoryLimit uint64                `json:"memoryLimit"`
	ProcLimit   uint64                `json:"procLimit"`
	CopyIn      map[string]judgeFile  `json:"copyIn,omitempty"`
}

type judgeRequest struct {
	Cmd []judgeCmd `json:"cmd"`
}

type judgeResult struct {
	Status     string            `json:"status"`
	ExitStatus int               `json:"exitStatus"`
	Error      string            `json:"error,omitempty"`
	Files      map[string]string `json:"files,omitempty"`
}

type judgeResponse []judgeResult

type langConfig struct {
	srcName     string
	compileArgs []string
	runArgs     []string
	needCompile bool
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
	},
	"c": {
		srcName:     "prog.c",
		compileArgs: []string{"gcc", "-O2", "-Wall", "prog.c", "-o", "prog"},
		runArgs:     []string{"./prog"},
		needCompile: true,
	},
	"cpp": {
		srcName:     "prog.cpp",
		compileArgs: []string{"g++", "-O2", "-Wall", "prog.cpp", "-o", "prog"},
		runArgs:     []string{"./prog"},
		needCompile: true,
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

func (e *Executor) Execute(req *ExecuteRequest) (*ExecuteResponse, error) {
	cfg, ok := supportedLangs[req.Lang]
	if !ok {
		return nil, fmt.Errorf("不支持的语言: %s", req.Lang)
	}

	env := []string{"PATH=/usr/bin:/usr/local/bin:/usr/lib/jvm/java-11-openjdk/bin:/usr/local/go/bin"}

	common := judgeCmd{
		Env:         env,
		Files:       buildFiles(req.Stdin),
		CPULimit:    10000000000,
		MemoryLimit: 268435456,
		ProcLimit:   50,
	}

	var cmds []judgeCmd

	if cfg.needCompile {
		compileCmd := common
		compileCmd.Args = cfg.compileArgs
		compileCmd.CopyIn = map[string]judgeFile{
			cfg.srcName: {Content: strPtr(req.Src)},
		}
		cmds = append(cmds, compileCmd)

		runCmd := common
		runCmd.Args = cfg.runArgs
		cmds = append(cmds, runCmd)
	} else {
		runCmd := common
		runCmd.Args = cfg.runArgs
		runCmd.CopyIn = map[string]judgeFile{
			cfg.srcName: {Content: strPtr(req.Src)},
		}
		cmds = append(cmds, runCmd)
	}

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

	if cfg.needCompile && len(judgeResp) >= 2 {
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
