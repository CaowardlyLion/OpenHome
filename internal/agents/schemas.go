package agents

func object(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
}

func RouteSchema() map[string]any {
	return object(map[string]any{
		"intent":       map[string]any{"type": "string"},
		"lane":         map[string]any{"type": "string", "enum": []string{string(DirectAnswer), string(SimpleTask), string(PlannedTask)}},
		"verification": map[string]any{"type": "string", "enum": []string{string(VerifyNone), string(VerifyFinal)}},
		"reason":       map[string]any{"type": "string"},
	}, "intent", "lane", "verification", "reason")
}

func PlanSchema() map[string]any {
	return object(map[string]any{
		"summary": map[string]any{"type": "string"},
		"steps": map[string]any{"type": "array", "minItems": 1, "items": object(map[string]any{
			"id": map[string]any{"type": "string"}, "goal": map[string]any{"type": "string"}, "successCriteria": map[string]any{"type": "string"},
		}, "id", "goal", "successCriteria")},
	}, "summary", "steps")
}

func SkillChoiceSchema() map[string]any {
	return object(map[string]any{
		"skillName": map[string]any{"type": "string"}, "reason": map[string]any{"type": "string"},
	}, "skillName", "reason")
}

func VerificationSchema() map[string]any {
	return object(map[string]any{
		"status": map[string]any{"type": "string", "enum": []string{"passed", "failed"}},
		"reason": map[string]any{"type": "string"},
	}, "status", "reason")
}

func CompletionSchema() map[string]any {
	return object(map[string]any{
		"summary": map[string]any{"type": "string"},
		"details": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
	}, "summary", "details")
}
