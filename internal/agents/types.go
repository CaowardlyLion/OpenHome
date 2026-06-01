package agents

import "encoding/json"

type Lane string

const (
	DirectAnswer Lane = "direct_answer"
	SimpleTask   Lane = "simple_task"
	PlannedTask  Lane = "planned_task"
)

type VerificationPolicy string

const (
	VerifyNone  VerificationPolicy = "none"
	VerifyFinal VerificationPolicy = "final"
)

type RouteDecision struct {
	Intent       string             `json:"intent"`
	Lane         Lane               `json:"lane"`
	Verification VerificationPolicy `json:"verification"`
	Reason       string             `json:"reason"`
}

type Plan struct {
	Summary string `json:"summary"`
	Steps   []Step `json:"steps"`
}

type Step struct {
	ID              string `json:"id"`
	Goal            string `json:"goal"`
	SuccessCriteria string `json:"successCriteria"`
}

type SkillChoice struct {
	SkillName string `json:"skillName"`
	Reason    string `json:"reason"`
}

func (choice *SkillChoice) UnmarshalJSON(content []byte) error {
	var decoded struct {
		SkillName      string `json:"skillName"`
		SkillNameSnake string `json:"skill_name"`
		Name           string `json:"name"`
		Reason         string `json:"reason"`
		SkillChoice    *struct {
			SkillName      string `json:"skillName"`
			SkillNameSnake string `json:"skill_name"`
			Name           string `json:"name"`
			Reason         string `json:"reason"`
		} `json:"skill_choice"`
	}
	if err := json.Unmarshal(content, &decoded); err != nil {
		return err
	}
	choice.SkillName = decoded.SkillName
	if choice.SkillName == "" {
		choice.SkillName = decoded.SkillNameSnake
	}
	if choice.SkillName == "" {
		choice.SkillName = decoded.Name
	}
	choice.Reason = decoded.Reason
	if choice.SkillName == "" && decoded.SkillChoice != nil {
		choice.SkillName = decoded.SkillChoice.SkillName
		if choice.SkillName == "" {
			choice.SkillName = decoded.SkillChoice.SkillNameSnake
		}
		if choice.SkillName == "" {
			choice.SkillName = decoded.SkillChoice.Name
		}
		choice.Reason = decoded.SkillChoice.Reason
	}
	return nil
}

type Verification struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type CompletionReport struct {
	Summary string   `json:"summary"`
	Details []string `json:"details"`
}

type StatusEvent struct {
	Type    string
	Message string
}
