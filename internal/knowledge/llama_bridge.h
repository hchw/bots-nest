#ifndef LLAMA_BRIDGE_H
#define LLAMA_BRIDGE_H

void* bridge_init(const char* model_path);
int   bridge_embed(void* state, const char* text, float* out, int* out_dim);
int   bridge_dim(void* state);
void  bridge_free(void* state);

#endif
