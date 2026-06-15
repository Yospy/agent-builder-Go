package runtime

import (
	"encoding/json"
	"time"
)

type builderState struct {
	Version     int               `json:"version"`
	Draft       map[string]any    `json:"draft"`
	Answered    []string          `json:"answered"`
	Assumptions map[string]string `json:"assumptions"`
	Questions   []builderQuestion `json:"questions"`
}

type builderQuestion struct {
	CallID     string `json:"call_id"`
	Field      string `json:"field"`
	Question   string `json:"question"`
	Answer     string `json:"answer,omitempty"`
	AnsweredAt int64  `json:"answered_at,omitempty"`
}

func parseBuilderState(raw string) builderState {
	var st builderState
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &st)
	}
	if st.Version == 0 {
		st.Version = 1
	}
	if st.Draft == nil {
		st.Draft = map[string]any{}
	}
	if st.Answered == nil {
		st.Answered = []string{}
	}
	if st.Assumptions == nil {
		st.Assumptions = map[string]string{}
	}
	if st.Questions == nil {
		st.Questions = []builderQuestion{}
	}
	return st
}

func (s *builderState) recordAnswer(callID, field, question, answer, value string) {
	if s.Version == 0 {
		s.Version = 1
	}
	if s.Draft == nil {
		s.Draft = map[string]any{}
	}
	s.Draft[field] = value
	if !containsString(s.Answered, field) {
		s.Answered = append(s.Answered, field)
	}
	now := time.Now().Unix()
	for i := range s.Questions {
		if s.Questions[i].CallID == callID {
			s.Questions[i].Answer = answer
			s.Questions[i].AnsweredAt = now
			return
		}
	}
	s.Questions = append(s.Questions, builderQuestion{
		CallID:     callID,
		Field:      field,
		Question:   question,
		Answer:     answer,
		AnsweredAt: now,
	})
}

func (s builderState) json() string {
	b, err := json.Marshal(s)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
