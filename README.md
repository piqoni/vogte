 ```
 ██╗   ██╗  ██████╗   ██████╗ ████████╗ ███████╗
 ██║   ██║ ██╔═══██╗ ██╔════╝ ╚══██╔══╝ ██╔════╝
 ██║   ██║ ██║   ██║ ██║  ███╗   ██║    █████╗
 ╚██╗ ██╔╝ ██║   ██║ ██║   ██║   ██║    ██╔══╝
  ╚████╔╝  ╚██████╔╝ ╚███████║   ██║    ███████╗
   ╚═══╝    ╚═════╝   ╚══════╝   ╚═╝    ╚══════╝
❯ Your agentic terminal for existing Go codebases.
```
# Motivation
This is an attempt to create a language-specific tool that provides holistic repository context and helps developers build and maintain Go projects using LLMs. Some potential benefits of being language-specific:
  - Easier parsing of the repository using Abstract Syntax Trees to extract more relevant/compressed information for LLM context
  - Utilizing Go tooling to its fullest to help validate changes with the fastest feedback (running tests, go vet, goimports, etc.)
  - Applying patches from LLMs in a more robust way (potentially using AST trees - still exploring; currently it's line-based)
  - When the language is assumed, there is near-zero configuration (for example, if you have OPENAI_API_KEY defined, vogte just works with no configuration)

# Features
 - Holistic repository context ("compressed" AST that helps the LLM decide the best way to tackle the task)
 - Ask/Agent mode (Agent mode means it can apply patches directly - still rough around the edges; AST approach being explored)
 - Runs "Sanity Check" after patching and displays project health 🟢 for instant feedback (currently "go vet ./...", but additional checks will be added eventually)
 - Tested with GPT-4 and Claude Sonnet (but any OpenAI-compatible API should work)
 - CLI mode that can produce holistic context

# Non-Features
- Every message is considered a new chat and not related to the previous. The idea is to provide all what is needed in one go; this is also (likely) more cost effective.
- There is no agentic loop on fail (at least for now).

# How it works
Vogte uses a two-step approach for providing tasks to the LLM. In the first step, it extracts relevant parts (structs/interfaces/methods along with signatures) from your repository and asks the LLM which files it needs in full to solve the problem expressed by the user. During this step, the LLM returns a list of files, which vogte then provides back with their full content so the LLM can apply the solution.

# Install

 go install github.com/piqoni/vogte/cmd/vogte@latest

# Usage
If you have set OPENAI_API_KEY in your system and if you want to use GPT-5 then just start vogte in the directory you are interested to work on and it will use GPT-5 automatically.
If you want to use any of the Claude models, make sure you have setup ANTHROPIC_API_KEY and start vogte with **vogte -model claude-sonnet-4-0**

Options:
```
  -agent
    	Start in AGENT mode
  -config string
    	Path to config file
  -dir string
    	The directory to analyze/apply changes to
  -model string
    	LLM model name (overrides config)
  -output string
    	The output file (default "vogte-output.txt")
```

# No LLM API? No Problem.
If you want to generate just the "compressed repository context of your project" so you could use it in LLMs via web ui, you can generate using:
**vogte -output**
