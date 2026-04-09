package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	// Assumes a `go.mod` file defines the module name (e.g., 'shell')
	"shell/presets"

	"github.com/charmbracelet/huh"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type AppConfig struct {
	Provider string
	Model    string
	APIKey   string
}

// AIRegistry represents a dynamic list of providers and models
type AIRegistry struct {
	Providers []ProviderConfig `json:"providers"`
}

type ProviderConfig struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Models []string `json:"models"`
}

var (
	aiClient *genai.Client
	config   = AppConfig{
		Provider: "google",
		Model:    "gemini-1.5-flash",
		APIKey:   os.Getenv("GEMINI_API_KEY"),
	}
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	ctx := context.Background()

	// Initialize the AI client once at startup for better performance and resource management
	if config.APIKey != "" {
		var err error
		aiClient, err = genai.NewClient(ctx, option.WithAPIKey(config.APIKey))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating AI client: %v\n", err)
		}
	}
	defer func() { if aiClient != nil { aiClient.Close() } }()

	fmt.Println("Welcome to AI Shell. Type natural language, native commands, '/model' for AI setup, '/settings' for preferences, or 'exit' to quit.")

	for {
		// The native terminal prompt
		cwd, _ := os.Getwd()
		fmt.Printf("\033[36mai-shell %s>\033[0m ", cwd)

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if strings.ToLower(input) == "exit" {
			break
		}

		// Handle the settings menu
		if strings.HasPrefix(input, "/settings") {
			handleSettings()
			continue
		}

		// Handle the model config menu
		if strings.HasPrefix(input, "/model") {
			handleModelConfig(ctx)
			continue
		}

		// Check for a preset command first for performance and to bypass AI
		if command, found := presets.CheckForPreset(input); found {
			fmt.Printf("\033[32m-> %s\033[0m\n", command) // Show the resolved command
			// Presets are considered safe and are executed directly
			executeCommand(command)
			continue // Skip AI and go to next prompt
		}

		// Show a premium loading spinner while waiting for the AI
		done := make(chan bool)
		go func() {
			chars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			i := 0
			for {
				select {
				case <-done:
					// Clear the spinner line completely
					fmt.Print("\r\033[K")
					return
				default:
					fmt.Printf("\r\033[35m%s Translating...\033[0m", chars[i])
					i = (i + 1) % len(chars)
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()

		// 1. Send English text to real AI for translation
		command, isSafe := generateCommandFromAI(ctx, input)

		// Stop the spinner
		done <- true

		// Visually display the generated command to simulate the "replacement" feel
		fmt.Printf("\033[32m-> %s\033[0m\n", command)

		// 2. Safety Verification
		if !isSafe {
			fmt.Print("\033[33m⚠️  Command might be unsafe. Execute? (y/n):\033[0m ")
			confirm, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				fmt.Println("Execution cancelled.")
				continue
			}
		}

		// 3. Execution
		executeCommand(command)
	}
}

func handleSettings() {
	fmt.Println("\n--- ⚙️ Settings ---")
	fmt.Println("1. Auto-execution: ON (Ask only on unsafe)")
	fmt.Println("2. AI API Configuration (Not configured)")
	fmt.Println("3. Mode: English + Native integrated")
	fmt.Println("(Note: Settings interactive menu to be implemented)\n")
}

func handleModelConfig(ctx context.Context) {
	fmt.Print("\n\033[36mFetching latest AI providers and models...\033[0m\n")
	registry := fetchAIRegistry()

	var providerOptions []huh.Option[string]
	for _, prov := range registry.Providers {
		providerOptions = append(providerOptions, huh.NewOption(prov.Name, prov.ID))
	}

	p := config.Provider
	
	// Step 1: Provider Selection
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("AI Provider").
				Options(providerOptions...).
				Value(&p),
		),
	).Run()

	if err != nil {
		fmt.Println("\n\033[31mConfiguration cancelled:\033[0m", err)
		return
	}

	// Step 2: Dynamic Model Options based on Provider
	var modelOptions []huh.Option[string]
	for _, prov := range registry.Providers {
		if prov.ID == p {
			for _, mod := range prov.Models {
				modelOptions = append(modelOptions, huh.NewOption(mod, mod))
			}
			break
		}
	}
	modelOptions = append(modelOptions, huh.NewOption("Custom (Type manually)", "custom"))

	m := config.Model
	k := config.APIKey

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Model").
				Description("Type to search/filter models...").
				Height(10).
				Options(modelOptions...).
				Value(&m),
			huh.NewInput().
				Title("API Key").
				EchoMode(huh.EchoModePassword).
				Value(&k),
		),
	).Run()

	if err != nil {
		fmt.Println("\n\033[31mConfiguration cancelled:\033[0m", err)
		return
	}
	
	if m == "custom" {
		m = ""
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Enter Custom Model Name").
					Value(&m),
			),
		).Run()
		if err != nil {
			fmt.Println("\n\033[31mConfiguration cancelled:\033[0m", err)
			return
		}
	}

	config.Provider, config.Model, config.APIKey = p, m, k

	if config.Provider == "google" {
		if aiClient != nil {
			aiClient.Close()
		}
		if config.APIKey != "" {
			client, err := genai.NewClient(ctx, option.WithAPIKey(config.APIKey))
			if err != nil {
				fmt.Printf("\n\033[31mError creating AI client:\033[0m %v\n", err)
			} else {
				aiClient = client
				fmt.Println("\n\033[32mSuccessfully updated Google Gemini configuration.\033[0m")
			}
		}
	} else {
		fmt.Printf("\n\033[33mConfiguration saved. Note: %s integration is currently a placeholder.\033[0m\n", config.Provider)
	}
}

// fetchAIRegistry retrieves the native providers and dynamically fetches OpenRouter's list
func fetchAIRegistry() AIRegistry {
	registry := AIRegistry{
		Providers: []ProviderConfig{
			{ID: "google", Name: "Google", Models: []string{"gemini-1.5-flash", "gemini-1.5-pro", "gemini-1.0-pro"}},
			{ID: "openai", Name: "OpenAI", Models: []string{"gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo", "gpt-4o-mini"}},
			{ID: "anthropic", Name: "Anthropic", Models: []string{"claude-3-5-sonnet-20240620", "claude-3-opus-20240229"}},
			{ID: "vertex", Name: "Vertex AI", Models: []string{"gemini-1.5-flash", "gemini-1.5-pro", "gemini-1.0-pro"}},
		},
	}

	client := &http.Client{Timeout: 5 * time.Second}
	// Fetching real-time models from OpenRouter (no API key required for this endpoint)
	resp, err := client.Get("https://openrouter.ai/api/v1/models")
	if err != nil || resp.StatusCode != 200 {
		return registry
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return registry
	}

	var orResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &orResp); err != nil {
		return registry
	}

	var orModels []string
	dynamicModels := make(map[string][]string)

	for _, m := range orResp.Data {
		orModels = append(orModels, m.ID)

		// Use OpenRouter to dynamically discover native models
		parts := strings.SplitN(m.ID, "/", 2)
		if len(parts) == 2 {
			dynamicModels[parts[0]] = append(dynamicModels[parts[0]], parts[1])
		}
	}

	for i, prov := range registry.Providers {
		if models, ok := dynamicModels[prov.ID]; ok && len(models) > 0 {
			registry.Providers[i].Models = models
		}
	}

	if len(orModels) > 0 {
		registry.Providers = append(registry.Providers, ProviderConfig{
			ID:     "openrouter",
			Name:   "OpenRouter",
			Models: orModels,
		})
	}

	return registry
}

// generateCommandFromAI uses the configured AI API to translate natural language into commands
func generateCommandFromAI(ctx context.Context, input string) (string, bool) {
	if config.Provider != "google" {
		return fmt.Sprintf("echo \033[31mError: Generation for provider '%s' is not implemented yet. Please use '/model' to switch to Google.\033[0m", config.Provider), true
	}

	if aiClient == nil {
		return "echo \033[31mError: AI client is not configured. Please use '/model' to set your API key.\033[0m", true
	}

	model := aiClient.GenerativeModel(config.Model)
	model.SetTemperature(0.1) // Lower temperature for more deterministic outputs
	
	cwd, _ := os.Getwd()
	prompt := fmt.Sprintf(`You are a lightweight AI shell assistant for %s. 
Translate the user's natural language into a valid %s terminal command.
The current working directory is: %s
If the input is already a valid command, return it as is.
Return ONLY a raw JSON object. Do not wrap it in markdown block quotes.
Format: {"command": "the command to run", "is_safe": true/false}
Set is_safe to false ONLY for destructive/dangerous commands (e.g., delete, format, rmdir).

User input: %s`, runtime.GOOS, runtime.GOOS, cwd, input)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil || len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "echo \033[31mAI Error: unable to generate a response\033[0m", true
	}

	textResponse, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return "echo \033[31mAI Error: unexpected response format\033[0m", true
	}

	// Clean potential markdown codeblock formatting that the AI might forcefully inject
	cleanJSON := strings.TrimPrefix(strings.TrimSpace(string(textResponse)), "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")

	var aiResp struct {
		Command string `json:"command"`
		IsSafe  bool   `json:"is_safe"`
	}

	if err := json.Unmarshal([]byte(cleanJSON), &aiResp); err != nil {
		return fmt.Sprintf("echo \033[31mError parsing AI JSON response:\033[0m %v", err), false
	}

	return aiResp.Command, aiResp.IsSafe
}

func executeCommand(cmdStr string) {
	if cmdStr == "" {
		return
	}

	// Internal handling for "cd" to preserve directory state within the Go application
	if strings.HasPrefix(cmdStr, "cd ") || cmdStr == "cd" || strings.HasPrefix(cmdStr, "cd..") {
		dir := strings.TrimSpace(strings.TrimPrefix(cmdStr, "cd"))
		if dir == ".." {
			dir = ".."
		} else if dir == "" {
			if runtime.GOOS == "windows" {
				cwd, _ := os.Getwd()
				fmt.Println(cwd)
			} else {
				home, _ := os.UserHomeDir()
				_ = os.Chdir(home)
			}
			return
		}
		
		dir = strings.Trim(dir, "\"'") // Remove quotes if any
		if err := os.Chdir(dir); err != nil {
			fmt.Printf("\033[31mcd failed:\033[0m %v\n", err)
		}
		return
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}
	
	// Bind standard streams so the output behaves exactly like a native terminal
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		fmt.Printf("\033[31mCommand failed:\033[0m %v\n", err)
	}
}