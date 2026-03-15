# Memory Overview
Amaros memory now has two layers:
1. **Markdown (episodic)** — task summaries are written to markdown files under the configured memory path so the agent can recall recent work.
2. **Chroma (semantic)** — a Chroma collection stores vectorised summaries for similarity search across broader context.

Configure paths and Chroma mode in `memory` settings inside `~/.config/amaros/config.yaml`. The llm_execute_with_memory example uses both layers to ground its decisions and updates memories when tasks complete.
