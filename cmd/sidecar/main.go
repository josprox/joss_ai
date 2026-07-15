package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type request struct {
	Protocol string        `json:"protocol"`
	ID       string        `json:"id"`
	Method   string        `json:"method"`
	Args     []interface{} `json:"args"`
}

type rpcError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type response struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *rpcError   `json:"error,omitempty"`
}

func main() {
	var req request
	if err := json.NewDecoder(io.LimitReader(os.Stdin, 16<<20)).Decode(&req); err != nil {
		write(response{Error: &rpcError{Code: "BAD_REQUEST", Message: err.Error()}})
		return
	}
	if req.Protocol != "joss-rpc-v1" {
		write(response{ID: req.ID, Error: &rpcError{Code: "BAD_PROTOCOL", Message: "se requiere joss-rpc-v1"}})
		return
	}
	if req.Method == "stream" {
		result, err := streamDispatch(req.ID, req.Args)
		if err != nil {
			write(response{ID: req.ID, Error: &rpcError{Code: "AI_STREAM_ERROR", Message: err.Error()}})
			return
		}
		write(response{ID: req.ID, Result: result})
		return
	}
	result, err := dispatch(req.Method, req.Args)
	if err != nil {
		write(response{ID: req.ID, Error: &rpcError{Code: "AI_ERROR", Message: err.Error()}})
		return
	}
	write(response{ID: req.ID, Result: result})
}

func streamDispatch(id string, args []interface{}) (string, error) {
	if len(args) != 3 {
		return "", fmt.Errorf("stream requiere provider, model y messages")
	}
	provider := strings.ToLower(strings.TrimSpace(fmt.Sprint(args[0])))
	model := strings.TrimSpace(fmt.Sprint(args[1]))
	if provider == "" || provider == "<nil>" {
		provider = envDefault("AI_PROVIDER", "groq")
	}
	if model == "" || model == "<nil>" {
		model = envDefault("AI_MODEL", "llama-3.1-8b-instant")
	}
	return streamChat(id, provider, model, args[2])
}

func streamChat(id, provider, model string, messages interface{}) (string, error) {
	var endpoint, keyName string
	switch provider {
	case "groq":
		endpoint, keyName = "https://api.groq.com/openai/v1/chat/completions", "GROQ_API_KEY"
	case "openai":
		endpoint, keyName = "https://api.openai.com/v1/chat/completions", "OPENAI_API_KEY"
	case "gemini":
		endpoint, keyName = "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions", "GEMINI_API_KEY"
	default:
		return "", fmt.Errorf("proveedor no soportado: %s", provider)
	}
	apiKey := strings.TrimSpace(os.Getenv(keyName))
	if apiKey == "" {
		return "", fmt.Errorf("falta %s", keyName)
	}
	body, _ := json.Marshal(map[string]interface{}{"model": model, "messages": messages, "stream": true})
	req, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: timeout()}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		return "", fmt.Errorf("%s HTTP %d: %s", provider, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	full := strings.Builder{}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var event map[string]interface{}
		if json.Unmarshal([]byte(data), &event) != nil {
			continue
		}
		choices, _ := event["choices"].([]interface{})
		if len(choices) == 0 {
			continue
		}
		choice, _ := choices[0].(map[string]interface{})
		delta, _ := choice["delta"].(map[string]interface{})
		content, _ := delta["content"].(string)
		if content == "" {
			continue
		}
		full.WriteString(content)
		write(map[string]interface{}{"id": id, "event": "chunk", "content": content})
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return full.String(), nil
}

func dispatch(method string, args []interface{}) (interface{}, error) {
	if method != "chat" && method != "content" {
		return nil, fmt.Errorf("método no soportado: %s", method)
	}
	if len(args) != 3 {
		return nil, fmt.Errorf("%s requiere provider, model y messages", method)
	}
	provider := strings.ToLower(strings.TrimSpace(fmt.Sprint(args[0])))
	model := strings.TrimSpace(fmt.Sprint(args[1]))
	if provider == "" || provider == "<nil>" {
		provider = envDefault("AI_PROVIDER", "groq")
	}
	if model == "" || model == "<nil>" {
		model = envDefault("AI_MODEL", "llama-3.1-8b-instant")
	}
	result, err := chat(provider, model, args[2])
	if err != nil || method == "chat" {
		return result, err
	}
	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("el proveedor no devolvió choices")
	}
	choice, _ := choices[0].(map[string]interface{})
	message, _ := choice["message"].(map[string]interface{})
	content, ok := message["content"].(string)
	if !ok {
		return nil, fmt.Errorf("el proveedor no devolvió message.content")
	}
	return content, nil
}

func chat(provider, model string, messages interface{}) (map[string]interface{}, error) {
	var endpoint, keyName string
	switch provider {
	case "groq":
		endpoint, keyName = "https://api.groq.com/openai/v1/chat/completions", "GROQ_API_KEY"
	case "openai":
		endpoint, keyName = "https://api.openai.com/v1/chat/completions", "OPENAI_API_KEY"
	case "gemini":
		endpoint, keyName = "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions", "GEMINI_API_KEY"
	default:
		return nil, fmt.Errorf("proveedor no soportado: %s", provider)
	}
	apiKey := strings.TrimSpace(os.Getenv(keyName))
	if apiKey == "" {
		return nil, fmt.Errorf("falta %s", keyName)
	}
	body, err := json.Marshal(map[string]interface{}{"model": model, "messages": messages})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: timeout()}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s HTTP %d: %s", provider, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("respuesta JSON inválida: %w", err)
	}
	return result, nil
}

func timeout() time.Duration {
	value := envDefault("AI_TIMEOUT_SECONDS", "60")
	var seconds int
	if _, err := fmt.Sscanf(value, "%d", &seconds); err != nil || seconds < 1 || seconds > 3600 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func write(value interface{}) {
	_ = json.NewEncoder(os.Stdout).Encode(value)
}
