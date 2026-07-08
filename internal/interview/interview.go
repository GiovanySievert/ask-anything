package interview

type Question struct {
	Question string `json:"question"`
}

type Evaluation struct {
	Score         int      `json:"score"`
	Feedback      string   `json:"feedback"`
	MissingPoints []string `json:"missing_points"`
	WeakTopics    []string `json:"weak_topics"`
	NextQuestion  string   `json:"next_question"`
}
