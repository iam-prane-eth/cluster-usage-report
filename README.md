# GO HTTP Function

Welcome to your new Go Function! The boilerplate function code can be found in `main.go`. This Function responds to HTTP requests.This function collects daily resource usage metrics from Prometheus (CPU, Memory, PVC, Node count etc) for Kubernetes clusters. It analyzes usage trends, detects anomalies, and generates a detailed, professional PDF report and uploads to minio integrated via Notify in tcl-security. The report includes:

- Visual graphs of resource utilization over time
- Markdown-driven AI analysis using OpenAI API
- Summarized tables and actionable recommendations

code blueprint:
- collects metris from prometheus
- update/create cm `monthly-metrics` with collected metrics against the date
- builds the graph with appropriate metrics
- send `montly-metrics` json to openai for analysis
- generates pdf though python
- send to minio using notify func

---

# Trigger

this code can be triggered in two ways:

- **Via PingSource (Automated Trigger)**:  
  When triggered by a PingSource, the function collects daily metrics and stores them in a ConfigMap `monthly-metrics`. On the last day of the month, it generates the complete PDF report and uploads it to Minio.

- **Via API(or TCx)**:  
  The tool can also be run manually, in which case it collects metrics from the same ConfigMap (from the 1st day of the month up to the specified day) and generates the PDF report immediately

Note: model names gets updated sometimes ,please verify if ksvc uses correct model by running following command

```bash
curl -X GET "https://api.ai-cloud.cloudlyte.com/v1/models"  -H "Authorization: Bearer sk-sBQX1aB8TOb--oXirsI05g" -k
```

```bash
curl -X POST \
-H "content-type: application/json"  \
-H "ce-specversion: 1.0"  \
-H "ce-source: curl-command"  \
-H "ce-type: curl.notify"  \
-H "ce-id: iks"  \
http://cluster-usage-report-service.tcl-security.svc.cluster.local
```

response from this will look like this 
```bash
{"name": "cluster-usage-report", "file": "report-2025-07-02.pdf"}
```
next trigger the notify

```bash
curl -X POST \
-H "content-type: application/json"  \
-H "ce-specversion: 1.0"  \
-H "ce-source: curl-command"  \
-H "ce-type: curl.notify"  \
-H "ce-id: iks"  \
-d '{"name": "cluster-usage-report", "file": "report-2025-07-02.pdf"}' \
http://notify-service.tcl-security.svc.cluster.local
```
we can trigger the sequence at once to automate this 
```bash
curl -X POST \
-H "content-type: application/json"  \
-H "ce-specversion: 1.0"  \
-H "ce-source: curl-command"  \
-H "ce-type: curl.notify"  \
-H "ce-id: iks"  \
-d '{"name": "cluster-usage-report", "file": "report-2025-07-02.pdf"}' \
http://cluster-usage-report-kn-sequence-0-kn-channel.tcl-security.svc.cluster.local
 
```
---

# Release Notes

27-11-2025: docker.io/shashankft/cluster-usage-report:v1
- changed cover page
- used python report labs for pdf generation

03-07-2025: docker.io/iampraneeth/cluster-usage-report:mdpdf-v16@sha256:aab22f3c7c95d174f66d8127981d71267399fe5972f6e201e00325975b2816b9

