package api

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestClusterSummaryUnmarshalJSON(t *testing.T) {
	data := `{
  "cluster": "exampleCluster",
  "latestReportSet": {
    "timestamp": "2015-10-21T07:28:42Z",
    "failureCount": 4,
    "successCount": 1,
    "reports": [
      {
        "id": "exampleReport1",
        "package": "examplePackage.ID.1",
        "failureCount": 2,
        "successCount": 1
      },
      {
        "id": "exampleReport2",
        "package": "examplePackage.ID.2",
        "failureCount": 2,
        "successCount": 0
      }
    ]
  }
}`
	ts, err := time.Parse(TimeFormat, "2015-10-21T07:28:42Z")
	if err != nil {
		t.Fatalf("%+v", err)
	}

	want := ClusterSummary{
		Cluster: "exampleCluster",
		LatestReportSet: &ReportSet{
			Cluster:      "exampleCluster",
			Timestamp:    Time{Time: ts},
			FailureCount: 4,
			SuccessCount: 1,
			Reports: []*ReportSummary{
				{
					ID:           "exampleReport1",
					Package:      "examplePackage.ID.1",
					Cluster:      "exampleCluster",
					Timestamp:    Time{Time: ts},
					FailureCount: 2,
					SuccessCount: 1,
				},
				{
					ID:           "exampleReport2",
					Package:      "examplePackage.ID.2",
					Cluster:      "exampleCluster",
					Timestamp:    Time{Time: ts},
					FailureCount: 2,
					SuccessCount: 0,
				},
			},
		},
	}

	var got ClusterSummary
	err = json.Unmarshal([]byte(data), &got)
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want: got=%+v, want=%+v", got, want)
	}
}
