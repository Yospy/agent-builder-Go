package runtime

import (
	"context"
	"net/http"
	"strings"
	"unicode"
)

type titleSummarizer interface {
	SummarizeTitle(ctx context.Context, prompt string) (string, error)
}

const chatTitleMaxWords = 3

type chatTitleReq struct {
	Prompt string `json:"prompt" validate:"required,max=4000"`
}

type chatTitleDTO struct {
	Title string `json:"title"`
}

func (s *Server) SetTitleSummarizer(ts titleSummarizer) {
	s.titleSummarizer = ts
}

func (s *Server) handleChatTitle(w http.ResponseWriter, r *http.Request) {
	var body chatTitleReq
	if !s.decode(w, r, &body) {
		return
	}
	if s.titleSummarizer == nil {
		s.writeJSON(w, http.StatusOK, chatTitleDTO{Title: compactTitleFallback(body.Prompt)})
		return
	}
	title, err := s.titleSummarizer.SummarizeTitle(r.Context(), body.Prompt)
	if err != nil {
		s.log.Warn("chat title summarizer failed", "err", err)
		title = compactTitleFallback(body.Prompt)
	}
	s.writeJSON(w, http.StatusOK, chatTitleDTO{Title: normalizeChatTitle(title, body.Prompt)})
}

func normalizeChatTitle(title, fallbackPrompt string) string {
	words := titleWords(title)
	if len(words) == 0 {
		return compactTitleFallback(fallbackPrompt)
	}
	if len(words) > chatTitleMaxWords {
		words = words[:chatTitleMaxWords]
	}
	return strings.Join(words, " ")
}

func compactTitleFallback(prompt string) string {
	words := titleWords(prompt)
	if len(words) == 0 {
		return "New chat"
	}
	if len(words) > chatTitleMaxWords {
		words = words[:chatTitleMaxWords]
	}
	return strings.Join(words, " ")
}

func titleWords(input string) []string {
	fields := strings.Fields(input)
	words := make([]string, 0, chatTitleMaxWords)
	for _, field := range fields {
		clean := strings.TrimFunc(field, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if clean == "" {
			continue
		}
		words = append(words, clean)
		if len(words) == chatTitleMaxWords+1 {
			break
		}
	}
	return words
}
