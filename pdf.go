package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

func ConvertMarkdownToPDF(markdownPath , pdfFilePath , clustername string, isPingSource bool, day int) error{

	timestamp := time.Now()
	subTitle := fmt.Sprintf("%s %d", timestamp.Month().String()[:3], timestamp.Year())
	year, month := timestamp.Year(), timestamp.Month()
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)

	var analysisTimeline string

	if isPingSource {
		analysisTimeline = fmt.Sprintf("Analysis Timeline: %s%d - %s%d",
			month.String()[:3], firstDay.Day(),
			month.String()[:3], lastDay.Day(),
			)
	} else {
		analysisTimeline = fmt.Sprintf("Analysis Timeline: %s%d - %s%d",
			month.String()[:3], firstDay.Day(),
			month.String()[:3], day,
			)
	}
	summary := "An in-depth analysis of cloud resource overprovisioning and utilization in Kubernetes applications, with key insights into CPU, memory, PVC, and nodes utilization."

	//imagePath := "/etc/reports/cluster-usage-report/metrics_graph.png"
        
        cmd := exec.Command("python3", "md_to_pdf.py", markdownPath, pdfFilePath,subTitle,summary,analysisTimeline,clustername)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Converting Markdown to PDF via Python: %s -> %s", markdownPath, pdfFilePath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run python script: %v", err)
	}

	log.Printf("PDF report successfully created: %s", pdfFilePath)
	return nil

}
