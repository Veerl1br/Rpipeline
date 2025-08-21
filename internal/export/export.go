package export

import (
	"encoding/json"
	"fmt"
	"os"

	rpipeline "github.com/Veerl1br/Rpipeline"
)

func ExportJSON(data []rpipeline.Result) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = os.WriteFile("results.json", content, 0644)
	if err != nil {
		return err
	}
	fmt.Println(data)
	return nil
}
