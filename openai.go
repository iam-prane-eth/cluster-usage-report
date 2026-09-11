package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func analyzeWithOpenAI(metricsData map[string]map[string]float64,BASE_URL string,Model string,API_Key string) (string, error) {
	client := openai.NewClient(
		option.WithAPIKey(API_Key),
		option.WithBaseURL(BASE_URL),
	)

	jsonString, err := json.Marshal(metricsData)
	if err != nil {
		fmt.Println(err)
		return "", err
	}


	chatCompletion, err := client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(`Analyze the following historical Prometheus metrics JSON, which contains date-over-date system performance data. Identify trends, spikes, or anomalies in CPU usage, memory consumption, PVC utilization, and node count over time.

					 Compare each day's data against the previous day and highlight significant changes — including fluctuations in number of nodes.
					 
					 Provide a structured day-on-day summary in tabular format, with the following columns:
					
					 Date | CPU Peak Usage (%) | Memory Peak Consumption (%)  | PVC Peak Utilization (%) | Node Count
					 
					 Use bold section headers to visually distinguish sections. Present an overall system health assessment and call out days with unusual patterns (e.g., sudden spikes or drops in any metric).
					 
					 Additionally, provide the average values of CPU, memory, PVC utilization from the averages section (supplied separately in the input), and compare these to daily values,and clearly mention the averages as period average in table headings
					 
					 If a significant anomaly is observed in any metric (e.g., sudden increase in PVC usage, drop in node count), identify possible causes and recommend appropriate actions — such as scaling up resources, optimizing workloads, or investigating potential failures.`),
			openai.UserMessage(string(jsonString)),
		},
		Model: Model,
		Temperature:     openai.Float(0.9),
                TopP:            openai.Float(0.9),
                MaxTokens: openai.Int(4000),
	})

	if err != nil {
		fmt.Println(err)
		return "", err
	}
	return chatCompletion.Choices[0].Message.Content, nil
}
