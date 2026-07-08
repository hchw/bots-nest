package knowledge

/*
#cgo CPPFLAGS: -I${SRCDIR}/../../third_party/go-llama.cpp/llama.cpp/include -I${SRCDIR}/../../third_party/go-llama.cpp/llama.cpp/ggml/include
#cgo LDFLAGS: -L${SRCDIR}/../../third_party/go-llama.cpp/llama.cpp/build/src
#cgo LDFLAGS: -L${SRCDIR}/../../third_party/go-llama.cpp/llama.cpp/build/common
#cgo LDFLAGS: -L${SRCDIR}/../../third_party/go-llama.cpp/llama.cpp/build/ggml/src
#cgo LDFLAGS: -lllama -lllama-common -lggml -lggml-base -lggml-cpu -lm -lstdc++ -lpthread -lgomp
#include <stdlib.h>
#include "llama_bridge.h"
*/
import "C"
import (
	"fmt"
	"sync"
	"unsafe"
)

type BuiltinEmbedder struct {
	mu    sync.Mutex
	state unsafe.Pointer
	dim   int
}

func NewBuiltinEmbedder(modelPath string) (*BuiltinEmbedder, error) {
	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	state := C.bridge_init(cPath)
	if state == nil {
		return nil, fmt.Errorf("加载 embedding 模型失败")
	}
	dim := int(C.bridge_dim(state))
	return &BuiltinEmbedder{state: state, dim: dim}, nil
}

func (b *BuiltinEmbedder) Embed(providerID, model string, texts []string) ([][]float32, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == nil {
		return nil, fmt.Errorf("本地 embedding 模型未加载")
	}

	result := make([][]float32, len(texts))
	for i, text := range texts {
		emb := make([]float32, b.dim)
		var dim C.int
		cText := C.CString(text)
		ret := C.bridge_embed(b.state, cText, (*C.float)(unsafe.Pointer(&emb[0])), &dim)
		C.free(unsafe.Pointer(cText))
		if ret != 0 {
			return nil, fmt.Errorf("embedding 推理失败 (第%d条)", i)
		}
		result[i] = emb[:dim]
	}
	return result, nil
}

func (b *BuiltinEmbedder) Free() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state != nil {
		C.bridge_free(b.state)
		b.state = nil
	}
}
