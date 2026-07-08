extern "C" {
#include "llama_bridge.h"
}
#include <llama.h>
#include <cstring>
#include <cstdlib>
#include <cstdio>
#include <exception>

struct bridge_state {
    struct llama_model * model;
    struct llama_context * ctx;
    int n_embd;
};

extern "C" {

void* bridge_init(const char* model_path) {
    llama_backend_init();

    struct llama_model_params model_params = llama_model_default_params();
    struct llama_model * model = llama_model_load_from_file(model_path, model_params);
    if (!model) {
        fprintf(stderr, "bridge: failed to load model from %s\n", model_path);
        return NULL;
    }

    struct llama_context_params ctx_params = llama_context_default_params();
    ctx_params.n_ctx = 2048;
    ctx_params.n_batch = 2048;
    ctx_params.n_ubatch = 2048;
    ctx_params.embeddings = true;
    ctx_params.pooling_type = LLAMA_POOLING_TYPE_MEAN;

    struct llama_context * ctx = llama_init_from_model(model, ctx_params);
    if (!ctx) {
        fprintf(stderr, "bridge: failed to create context\n");
        llama_model_free(model);
        return NULL;
    }

    struct bridge_state * state = (struct bridge_state*) malloc(sizeof(struct bridge_state));
    state->model = model;
    state->ctx = ctx;
    state->n_embd = llama_model_n_embd(model);

    return state;
}

int bridge_embed(void* state_ptr, const char* text, float* out, int* out_dim) {
    if (!state_ptr || !text || !out || !out_dim) return 1;
    struct bridge_state * state = (struct bridge_state*) state_ptr;

    const struct llama_vocab * vocab = llama_model_get_vocab(state->model);
    if (!vocab) return 1;

    int n_tokens;
    try {
        n_tokens = llama_tokenize(vocab, text, (int32_t)strlen(text), NULL, 0, true, false);
    } catch (const std::exception & e) {
        fprintf(stderr, "bridge: llama_tokenize (query) threw: %s\n", e.what());
        return 1;
    }
    if (n_tokens <= 0) {
        n_tokens = -n_tokens;
    }
    if (n_tokens <= 0) return 1;

    llama_token * tokens = (llama_token*) malloc((size_t)n_tokens * sizeof(llama_token));
    if (!tokens) return 1;

    int actual;
    try {
        actual = llama_tokenize(vocab, text, (int32_t)strlen(text), tokens, n_tokens, true, false);
    } catch (const std::exception & e) {
        fprintf(stderr, "bridge: llama_tokenize (encode) threw: %s\n", e.what());
        free(tokens);
        return 1;
    }
    if (actual < 0) {
        int needed = -actual;
        free(tokens);
        tokens = (llama_token*) malloc((size_t)needed * sizeof(llama_token));
        if (!tokens) return 1;
        try {
            actual = llama_tokenize(vocab, text, (int32_t)strlen(text), tokens, needed, true, false);
        } catch (const std::exception & e) {
            fprintf(stderr, "bridge: llama_tokenize (retry) threw: %s\n", e.what());
            free(tokens);
            return 1;
        }
        if (actual < 0) { free(tokens); return 1; }
        n_tokens = actual;
    } else {
        n_tokens = actual;
    }

    int max_tokens = llama_n_ctx(state->ctx) < 1 ? 1 : llama_n_ctx(state->ctx);
    if (n_tokens > max_tokens) {
        n_tokens = max_tokens;
    }

    llama_batch batch = llama_batch_get_one(tokens, n_tokens);

    try {
        if (llama_decode(state->ctx, batch)) {
            free(tokens);
            return 1;
        }
    } catch (const std::exception & e) {
        fprintf(stderr, "bridge: llama_decode threw: %s\n", e.what());
        free(tokens);
        return 1;
    }

    const float * emb = llama_get_embeddings(state->ctx);
    if (!emb) {
        free(tokens);
        return 1;
    }

    int dim = state->n_embd;
    memcpy(out, emb, (size_t)dim * sizeof(float));
    *out_dim = dim;

    free(tokens);
    return 0;
}

int bridge_dim(void* state_ptr) {
    struct bridge_state * state = (struct bridge_state*) state_ptr;
    return state->n_embd;
}

void bridge_free(void* state_ptr) {
    if (!state_ptr) return;
    struct bridge_state * state = (struct bridge_state*) state_ptr;
    llama_free(state->ctx);
    llama_model_free(state->model);
    llama_backend_free();
    free(state);
}

} // extern "C"
