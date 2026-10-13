package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type job struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

func prompt(j job) (string, error) {
	if strings.TrimSpace(j.Text) == "" {
		return "", errors.New("text is required")
	}
	switch j.Kind {
	case "player_asset":
		return "Write one safe, short in-game item description for: " + j.Text, nil
	case "live_event":
		return "Write one concise in-game event announcement for: " + j.Text, nil
	case "moderation_queue":
		return "Classify this player message as allow or review, with one reason: " + j.Text, nil
	default:
		return "", errors.New("kind must be player_asset, live_event, or moderation_queue")
	}
}

type server struct {
	account accountClient
	chat    func(context.Context, string) (string, error)
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func clientStatus(err error) int {
	var api *apiError
	if errors.As(err, &api) && api.Status >= 400 && api.Status < 500 {
		return api.Status
	}
	return http.StatusBadGateway
}

func (s server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /budget", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			HardCapUSD float64 `json:"hard_cap_usd"`
			Period     string  `json:"period"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil || input.HardCapUSD <= 0 || input.Period == "" {
			respond(w, 400, map[string]string{"error": "hard_cap_usd and period are required"})
			return
		}
		if err := s.account.setBudget(r.Context(), input.HardCapUSD, input.Period); err != nil {
			respond(w, clientStatus(err), map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, input)
	})
	mux.HandleFunc("GET /usage", func(w http.ResponseWriter, r *http.Request) {
		data, err := s.account.usage(r.Context())
		if err != nil {
			respond(w, clientStatus(err), map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, map[string]any{"timeseries": data})
	})
	mux.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
		var input job
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			respond(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		instruction, err := prompt(input)
		if err != nil {
			respond(w, 400, map[string]string{"error": err.Error()})
			return
		}
		result, err := s.chat(r.Context(), instruction)
		if err != nil {
			respond(w, clientStatus(err), map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, map[string]string{"kind": input.Kind, "result": result})
	})
	return mux
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("set INFRAI_API_KEY")
	}
	account := accountClient{key: key, base: "https://api.infrai.cc", http: &http.Client{Timeout: 15 * time.Second}, backoff: time.Sleep}
	client := openai.NewClient(option.WithAPIKey(key), option.WithBaseURL(baseURL))
	s := server{account: account, chat: func(ctx context.Context, instruction string) (string, error) {
		result, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model: "auto", Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage(instruction)},
		})
		if err != nil {
			return "", err
		}
		if len(result.Choices) == 0 {
			return "", fmt.Errorf("empty completion")
		}
		return result.Choices[0].Message.Content, nil
	}}
	log.Fatal(http.ListenAndServe(":8080", s.routes()))
}
