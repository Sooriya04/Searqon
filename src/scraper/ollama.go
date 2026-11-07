package scraper

import (
	"src/utils"
)

func queryOllamaJSON(prompt string) ([]byte, error) {
	return utils.QueryOllamaJSON(prompt)
}

func queryOllama(prompt string) (string, error) {
	return utils.QueryOllama(prompt)
}
