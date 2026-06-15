package runtime

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"agent-builder/engine"
)

type questionArgs struct {
	Field             string           `json:"field"`
	Question          string           `json:"question"`
	Options           []questionOption `json:"options"`
	AllowCustom       bool             `json:"allow_custom"`
	CustomPlaceholder *string          `json:"custom_placeholder"`
	ProgressLabel     *string          `json:"progress_label"`
	ProgressCurrent   *float64         `json:"progress_current"`
	ProgressTotal     *float64         `json:"progress_total"`
}

type questionOption struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Description *string `json:"description"`
	Value       string  `json:"value"`
}

type answerReq struct {
	CallID     string `json:"call_id" validate:"required"`
	OptionID   string `json:"option_id"`
	CustomText string `json:"custom_text"`
}

type questionAnswer struct {
	Field    string `json:"field"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Value    string `json:"value"`
	OptionID string `json:"option_id,omitempty"`
	Custom   bool   `json:"custom"`
}

type pendingQuestion struct {
	args questionArgs
	ch   chan questionAnswer
}

type questions struct {
	mu sync.Mutex
	m  map[string]pendingQuestion
}

func newQuestions() *questions {
	return &questions{m: make(map[string]pendingQuestion)}
}

func (q *questions) register(key string, args questionArgs) chan questionAnswer {
	q.mu.Lock()
	defer q.mu.Unlock()
	ch := make(chan questionAnswer, 1)
	q.m[key] = pendingQuestion{args: args, ch: ch}
	return ch
}

func (q *questions) deliver(key string, req answerReq) (bool, int, string) {
	q.mu.Lock()
	p, ok := q.m[key]
	q.mu.Unlock()
	if !ok {
		return false, 0, ""
	}

	answer, status, msg := normalizeAnswer(p.args, req)
	if status != 0 {
		return true, status, msg
	}

	select {
	case p.ch <- answer:
		return true, 0, ""
	default:
		return true, 400, "question already answered"
	}
}

func (q *questions) drop(key string) {
	q.mu.Lock()
	delete(q.m, key)
	q.mu.Unlock()
}

func parseQuestionArgs(raw json.RawMessage) (questionArgs, error) {
	var args questionArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return questionArgs{}, fmt.Errorf("invalid question arguments: %w", err)
	}
	args.Field = strings.TrimSpace(args.Field)
	args.Question = strings.TrimSpace(args.Question)
	if args.Field == "" {
		return questionArgs{}, fmt.Errorf("field is required")
	}
	if args.Question == "" {
		return questionArgs{}, fmt.Errorf("question is required")
	}
	if len(args.Options) == 0 || len(args.Options) > 4 {
		return questionArgs{}, fmt.Errorf("options must contain 1-4 choices")
	}
	seen := map[string]bool{}
	for i := range args.Options {
		opt := &args.Options[i]
		opt.ID = strings.TrimSpace(opt.ID)
		opt.Label = strings.TrimSpace(opt.Label)
		opt.Value = strings.TrimSpace(opt.Value)
		if opt.ID == "" || opt.Label == "" || opt.Value == "" {
			return questionArgs{}, fmt.Errorf("each option needs id, label, and value")
		}
		if seen[opt.ID] {
			return questionArgs{}, fmt.Errorf("duplicate option id %q", opt.ID)
		}
		seen[opt.ID] = true
	}
	return args, nil
}

func questionEvent(args questionArgs) engine.Event {
	options := make([]engine.QuestionOption, 0, len(args.Options))
	for _, opt := range args.Options {
		description := ""
		if opt.Description != nil {
			description = *opt.Description
		}
		options = append(options, engine.QuestionOption{
			ID: opt.ID, Label: opt.Label, Description: description, Value: opt.Value,
		})
	}
	event := engine.Event{
		Field:       args.Field,
		Question:    args.Question,
		Options:     options,
		AllowCustom: args.AllowCustom,
	}
	if args.CustomPlaceholder != nil {
		event.CustomPlaceholder = *args.CustomPlaceholder
	}
	if args.ProgressLabel != nil || args.ProgressCurrent != nil || args.ProgressTotal != nil {
		progress := &engine.QuestionProgress{}
		if args.ProgressLabel != nil {
			progress.Label = *args.ProgressLabel
		}
		if args.ProgressCurrent != nil {
			progress.Current = int(*args.ProgressCurrent)
		}
		if args.ProgressTotal != nil {
			progress.Total = int(*args.ProgressTotal)
		}
		event.Progress = progress
	}
	return event
}

func normalizeAnswer(args questionArgs, req answerReq) (questionAnswer, int, string) {
	optionID := strings.TrimSpace(req.OptionID)
	customText := strings.TrimSpace(req.CustomText)
	if optionID == "" && customText == "" {
		return questionAnswer{}, 400, "option_id or custom_text is required"
	}
	if optionID != "" && customText != "" {
		return questionAnswer{}, 400, "provide either option_id or custom_text, not both"
	}
	if customText != "" {
		if !args.AllowCustom {
			return questionAnswer{}, 400, "custom answers are not allowed for this question"
		}
		return questionAnswer{
			Field: args.Field, Question: args.Question, Answer: customText, Value: customText, Custom: true,
		}, 0, ""
	}
	for _, opt := range args.Options {
		if opt.ID == optionID {
			return questionAnswer{
				Field: args.Field, Question: args.Question, Answer: opt.Label, Value: opt.Value, OptionID: opt.ID,
			}, 0, ""
		}
	}
	return questionAnswer{}, 400, "invalid option_id"
}

func answerJSON(answer questionAnswer) string {
	b, err := json.Marshal(answer)
	if err != nil {
		return `{"error":"could not encode answer"}`
	}
	return string(b)
}
