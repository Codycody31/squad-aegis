package utils

func GetString(data map[string]interface{}, key string) string {
	if v, ok := data[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func GetInt(data map[string]interface{}, key string) *int {
	if v, ok := data[key]; ok {
		if i, ok := v.(int); ok {
			return &i
		}
		if f, ok := v.(float64); ok {
			i := int(f)
			return &i
		}
	}
	return nil
}

func GetBool(data map[string]interface{}, key string) bool {
	if v, ok := data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
