package main

import (
	"sort"
	//"fmt"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"image/color"
	"log"
	"math"
)

func generateGraph(extractedMetrics map[string]map[string]float64, reportsPath string) {

	dates := make([]string, 0, len(extractedMetrics))
	for date := range extractedMetrics {
		dates = append(dates, date)
	}
	sort.Strings(dates) // Ensure chronological order

	p := plot.New()
	p.Title.Text = "Kubernetes Resource Utilization Over Time"
	p.X.Label.Text = "Days"
	p.Y.Label.Text = "Usage (%)"
	p.Add(plotter.NewGrid())

	p.X.Tick.Label.Rotation = math.Pi / 4 // Rotate X-axis labels 45°
	p.X.Tick.Label.XAlign = 0             // Align right
	p.X.Tick.Label.YAlign = -1            // Align top
	p.X.Tick.Label.Font.Size = vg.Points(8)

	// Prepare data points for plotting
	cpuVals, memVals, pvcVals, nodeVals := make(plotter.XYs, 0), make(plotter.XYs, 0), make(plotter.XYs, 0), make(plotter.XYs, 0)

	for i, date := range dates {
		metrics := extractedMetrics[date]
		cpuVals = append(cpuVals, plotter.XY{X: float64(i), Y: metrics["cpu_usage"]})
		memVals = append(memVals, plotter.XY{X: float64(i), Y: metrics["memory_usage"]})
		pvcVals = append(pvcVals, plotter.XY{X: float64(i), Y: metrics["pvc_utilization"]})
		nodeVals = append(nodeVals, plotter.XY{X: float64(i), Y: metrics["nodes"]})
	}

	addLine(p, cpuVals, color.RGBA{255, 0, 0, 255}, "CPU Usage")
	addLine(p, memVals, color.RGBA{0, 255, 0, 255}, "Memory Usage")
	addLine(p, pvcVals, color.RGBA{255, 165, 0, 255}, "PVC Utilization") // changed to orange for better contrast
	addLine(p, nodeVals, color.RGBA{0, 0, 255, 255}, "No of Nodes")

	p.X.Tick.Marker = plot.ConstantTicks(getXTicks(dates))

	if err := PrepareDir(reportsPath); err != nil {
		log.Fatalf("failed to create dir to save reports: %s", err)
	}

	// Save the plot to a file
	if err := p.Save(8*vg.Inch, 5*vg.Inch, "/etc/reports/cluster-usage-report/metrics_graph.png"); err != nil {
		log.Fatalf("Error in saving graph file %v", err)
	}

	log.Println("Graph saved as metrics_graph.png")

}

func addLine(p *plot.Plot, pts plotter.XYs, col color.RGBA, label string) {
	line, err := plotter.NewLine(pts)
	if err != nil {
		log.Println(err)
	}
	line.Color = col
	p.Add(line)
	p.Legend.Add(label, line)
}

func getXTicks(dates []string) []plot.Tick {
	ticks := make([]plot.Tick, len(dates))
	for i, date := range dates {
		ticks[i] = plot.Tick{Value: float64(i), Label: date[:6]} // Show "Feb-17" format
	}
	return ticks
}
