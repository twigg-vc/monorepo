package runnerlib

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseCiJobs(t *testing.T) {
	testCases := []struct {
		desc string

		// Use rawInput or input.
		// inputPayloads is used for writing more type safe tests - especially
		// for cases of when we want to test a non-ok rawInput but want to make sure
		// its for the reason we expect and not bc we forgot a ","
		rawInput string
		input    []CiJobJson

		ok                      bool
		expectNotOkMsgToContain string // use to check a specific error
		payloads                []CiJob
	}{
		{
			desc:     "empty",
			rawInput: "",
			ok:       false,
		},
		{
			desc:     "random string",
			rawInput: "dsafdasfdasfas",
			ok:       false,
		},
		{
			desc: "simple job",
			input: []CiJobJson{
				{
					Name: "say hi",
					Steps: []CiJobStepJson{
						{Run: "echo hi"},
					},
					TimeoutMilliSeconds: 1000,
				},
			},
			ok: true,
			payloads: []CiJob{
				{
					Job: JobPayload{
						Name: "say hi",
						Steps: []JobStep{
							{Run: "echo hi"},
						},
						TimeoutMilliSeconds: 1000},
				},
			},
		},
		{
			desc: "get-code template",
			rawInput: `
				{
					"Name": "get-code-with-step-template",
					"Steps": [
						{
							"TemplateName": "get-code"
						}
					],
					"TimeoutMilliSeconds": 1000
				}
			`,
			ok: true,
			payloads: []CiJob{
				{
					Job: JobPayload{
						Name: "get-code-with-step-template",
						Steps: []JobStep{
							{Dir: ".", Run: "tw init"},
							{Dir: ".", Run: "tw key $TWIGG_TOKEN"},
							{Dir: ".", Run: "tw server $REPO_ID"},
							{Dir: ".", Run: "tw pull $COMMIT_ID"},
						},
						TimeoutMilliSeconds: 1000},
				},
			},
		},
		{
			desc: "seconds timeout",
			rawInput: `
				{
					"Name": "say hi",
					"Steps": [
						{"Run": "echo hi"}
					],
					"TimeoutSeconds": 1
				}
			`,
			ok: true,
			payloads: []CiJob{
				{
					Job: JobPayload{
						Name: "say hi",
						Steps: []JobStep{
							{Run: "echo hi"},
						},
						TimeoutMilliSeconds: 1000,
					},
				},
			},
		},
		{
			desc: "minutes timeout",
			rawInput: `
				{
					"Name": "say hi",
					"Steps": [
						{"Run": "echo hi"}
					],
					"TimeoutMinutes": 1
				}
			`,
			ok: true,
			payloads: []CiJob{
				{
					Job: JobPayload{
						Name: "say hi",
						Steps: []JobStep{
							{Run: "echo hi"},
						},
						TimeoutMilliSeconds: 60000,
					},
				},
			},
		},
		{
			desc: "invalid seconds timeout",
			rawInput: `
				{
					"Name": "say hi",
					"Steps": [
						{"Run": "echo hi"}
					],
					"TimeoutSeconds": -1
				}
			`,
			ok:                      false,
			expectNotOkMsgToContain: "TimeoutSeconds",
		},
		{
			desc: "invalid minutes timeout",
			rawInput: `
				{
					"Name": "say hi",
					"Steps": [
						{"Run": "echo hi"}
					],
					"TimeoutMinutes": -1
				}
			`,
			ok:                      false,
			expectNotOkMsgToContain: "TimeoutMinutes",
		},
		{
			desc: "milis and seconds timeout",
			rawInput: `
				{
					"Name": "say hi",
					"Steps": [
						{"Run": "echo hi"}
					],
					"TimeoutMilliSeconds": 1000,
					"TimeoutSeconds": 1
				}
			`,
			ok:                      false,
			expectNotOkMsgToContain: "only one of (TimeoutMilliSeconds, TimeoutSeconds, TimeoutMinutes)",
		},
		{
			desc: "milis and minutes timeout",
			rawInput: `
				{
					"Name": "say hi",
					"Steps": [
						{"Run": "echo hi"}
					],
					"TimeoutMilliSeconds": 1000,
					"TimeoutMinutes": 1
				}
			`,
			ok:                      false,
			expectNotOkMsgToContain: "only one of (TimeoutMilliSeconds, TimeoutSeconds, TimeoutMinutes)",
		},
		{
			desc: "seconds and minutes timeout",
			rawInput: `
				{
					"Name": "say hi",
					"Steps": [
						{"Run": "echo hi"}
					],
					"TimeoutSeconds": 1000,
					"TimeoutMinutes": 1
				}
			`,
			ok:                      false,
			expectNotOkMsgToContain: "only one of (TimeoutMilliSeconds, TimeoutSeconds, TimeoutMinutes)",
		},
		{
			desc: "empty timeout",
			input: []CiJobJson{
				{
					Name: "say hi",
					Steps: []CiJobStepJson{
						{Run: "echo hi"},
					},
					// TimeoutMilliSeconds: 1000,
				},
			},
			ok: false,
		},
		{
			desc: "empty name",
			input: []CiJobJson{
				{
					// Name: "say hi",
					Steps: []CiJobStepJson{
						{Run: "echo hi"},
					},
					TimeoutMilliSeconds: 1000,
				},
			},
			ok: false,
		},
		{
			desc: "bad image",
			input: []CiJobJson{
				{
					Name:      "say hi",
					ImageName: "BAD NAME",
					Steps: []CiJobStepJson{
						{Run: "echo hi"},
					},
					TimeoutMilliSeconds: 1000,
				},
			},
			ok: false,
		},
		{
			desc: "duplicated name",
			input: []CiJobJson{
				{
					Name: "say hi",
					Steps: []CiJobStepJson{
						{Run: "echo hi"},
					},
					TimeoutMilliSeconds: 1000,
				},
				{
					Name: "say hi",
					Steps: []CiJobStepJson{
						{Run: "echo hi"},
					},
					TimeoutMilliSeconds: 1000,
				},
			},
			ok: false,
		},
		{
			desc: "step with secrets",
			input: []CiJobJson{
				{
					Name: "secret job",
					Steps: []CiJobStepJson{
						{
							Run:     "echo hi",
							Secrets: []string{"DB_PASS"},
						},
					},
					TimeoutMilliSeconds: 1000,
				},
			},
			ok: true,
			payloads: []CiJob{
				{
					Job: JobPayload{
						Name: "secret job",
						Steps: []JobStep{
							{
								Run:     "echo hi",
								Secrets: []string{"DB_PASS"},
							},
						},
						TimeoutMilliSeconds: 1000,
					},
				},
			},
		},
		{
			desc: "template with secrets",
			rawInput: `
				{
					"Name": "bad template secrets",
					"Steps": [
						{
							"TemplateName": "get-code",
							"Secrets": ["SECRET"]
						}
					],
					"TimeoutMilliSeconds": 1000
				}
			`,
			ok:                      false,
			expectNotOkMsgToContain: "Secrets",
		},
		{
			desc: "multiple secrets",
			input: []CiJobJson{
				{
					Name: "multi secret job",
					Steps: []CiJobStepJson{
						{
							Run:     "echo hi",
							Secrets: []string{"DB_PASS", "MY_KEY"},
						},
					},
					TimeoutMilliSeconds: 1000,
				},
			},
			ok: true,
			payloads: []CiJob{
				{
					Job: JobPayload{
						Name: "multi secret job",
						Steps: []JobStep{
							{
								Run:     "echo hi",
								Secrets: []string{"DB_PASS", "MY_KEY"},
							},
						},
						TimeoutMilliSeconds: 1000,
					},
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			if tC.rawInput != "" && len(tC.input) != 0 {
				panic("input and inputPayloads were both used")
			}
			if len(tC.input) > 0 {
				b, _ := json.Marshal(tC.input)
				tC.rawInput = string(b)
			}
			got, gotOk, notOkMsg := ParseCiJobs([]byte(tC.rawInput))
			if gotOk != tC.ok {
				t.Fatalf("%s failed: expected ok=%v got ok=%v notOkMsg=%s", tC.desc, tC.ok, gotOk, notOkMsg)
			}
			if gotOk && notOkMsg != "" {
				t.Fatalf("%s failed: expected got ok with non empty msg=%s", tC.desc, notOkMsg)
			}
			if !gotOk && notOkMsg == "" {
				t.Fatalf("%s failed: expected got notOk with empty msg", tC.desc)
			}
			if !gotOk {
				if tC.expectNotOkMsgToContain != "" {
					if !strings.Contains(notOkMsg, tC.expectNotOkMsgToContain) {
						t.Fatalf("%s failed: notOkMsg=%s doesnt contain %q",
							tC.desc, notOkMsg, tC.expectNotOkMsgToContain)
					}
				}
				return
			}
			if !reflect.DeepEqual(got, tC.payloads) {
				t.Fatalf("%s failed: expected payloads %#v got %#v", tC.desc, tC.payloads, got)
			}
		})
	}
}

func TestParseCdJobs(t *testing.T) {
	testCases := []struct {
		desc string

		rawInput string
		input    []CdJobJson

		ok                      bool
		expectNotOkMsgToContain string // use to check a specific error
		expectedJobs            []CdJob
	}{
		{
			desc:     "empty",
			rawInput: "",
			ok:       false,
		},
		{
			desc:     "random string",
			rawInput: "dsafdasfdasfas",
			ok:       false,
		},
		{
			desc: "simple job",
			input: []CdJobJson{
				{
					Name: "simple job",
					On:   []JobTrigger{"submit"},
					Stages: []CdJobStageJson{
						{
							CanAutoStart: true,
							Name:         "first stage",
							ImageName:    "go",
							Steps: []CiJobStepJson{
								{Run: "echo hi"},
							},
							TimeoutSeconds: 1,
						},
					},
				},
			},
			ok: true,
			expectedJobs: []CdJob{
				{
					Name: "simple job",
					On:   []JobTrigger{"submit"},
					Stages: []CdJobPayload{
						{
							CanAutoStart: true,
							JobPayload: JobPayload{
								Name:      "first stage",
								ImageName: "go",
								Steps: []JobStep{
									{Run: "echo hi"},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
					},
				},
			},
		},
		{
			desc: "simple job with raw input",
			rawInput: `
				{
					"Name": "simple job",
					"On": ["submit"],
					"Stages": [
						{
						"CanAutoStart": true,
						"Name": "first stage",
						"ImageName": "go",
						"Steps": [
							{
								"Run": "echo hi"
							}
						],
						"TimeoutMinutes": 1
						}
					]
				}
			`,
			ok: true,
			expectedJobs: []CdJob{
				{
					Name: "simple job",
					On:   []JobTrigger{"submit"},
					Stages: []CdJobPayload{
						{
							CanAutoStart: true,
							JobPayload: JobPayload{
								Name:      "first stage",
								ImageName: "go",
								Steps: []JobStep{
									{Run: "echo hi"},
								},
								TimeoutMilliSeconds: 60_000,
							},
						},
					},
				},
			},
		},
		{
			desc: "simple job-list with raw input",
			rawInput: `
			[
				{
					"Name": "say-hi-on-vm",
					"On": ["manual"],
					"Stages": [
						{
							"CanAutoStart": true,
							"Name": "say hi",
							"ImageName": "vm",
							"Steps": [
								{
									"Run": "echo hi"
								}
							],
							"TimeoutMinutes": 1
						}
					]
				}
			]
			`,
			ok: true,
			expectedJobs: []CdJob{
				{
					Name: "say-hi-on-vm",
					On:   []JobTrigger{"manual"},
					Stages: []CdJobPayload{
						{
							CanAutoStart: true,
							JobPayload: JobPayload{
								Name:      "say hi",
								ImageName: "vm",
								Steps: []JobStep{
									{Run: "echo hi"},
								},
								TimeoutMilliSeconds: 60_000,
							},
						},
					},
				},
			},
		},
		{
			desc: "use template",
			input: []CdJobJson{
				{
					Name: "simple job",
					Stages: []CdJobStageJson{
						{
							CanAutoStart: true,
							Name:         "first stage",
							ImageName:    "go",
							Steps: []CiJobStepJson{
								{TemplateName: "get-code"},
							},
							TimeoutMilliSeconds: 500,
						},
					},
				},
			},
			ok: true,
			expectedJobs: []CdJob{
				{
					Name: "simple job",
					Stages: []CdJobPayload{
						{
							CanAutoStart: true,
							JobPayload: JobPayload{
								Name:      "first stage",
								ImageName: "go",
								Steps: []JobStep{
									{Dir: ".", Run: "tw init"},
									{Dir: ".", Run: "tw key $TWIGG_TOKEN"},
									{Dir: ".", Run: "tw server $REPO_ID"},
									{Dir: ".", Run: "tw pull $COMMIT_ID"},
								},
								TimeoutMilliSeconds: 500,
							},
						},
					},
				},
			},
		},
		{
			desc: "stage step with secrets",
			input: []CdJobJson{
				{
					Name: "deploy job",
					Stages: []CdJobStageJson{
						{
							CanAutoStart: true,
							Name:         "deploy stage",
							ImageName:    "go",
							Steps: []CiJobStepJson{
								{
									Run:     "deploy",
									Secrets: []string{"DEPLOY_KEY"},
								},
							},
							TimeoutSeconds: 1,
						},
					},
				},
			},
			ok: true,
			expectedJobs: []CdJob{
				{
					Name: "deploy job",
					Stages: []CdJobPayload{
						{
							CanAutoStart: true,
							JobPayload: JobPayload{
								Name:      "deploy stage",
								ImageName: "go",
								Steps: []JobStep{
									{
										Run:     "deploy",
										Secrets: []string{"DEPLOY_KEY"},
									},
								},
								TimeoutMilliSeconds: 1000,
							},
						},
					},
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			if tC.rawInput != "" && len(tC.input) != 0 {
				panic("input and inputPayloads were both used")
			}
			if len(tC.input) > 0 {
				b, _ := json.Marshal(tC.input)
				tC.rawInput = string(b)
			}
			got, gotOk, notOkMsg := ParseCdJobs([]byte(tC.rawInput))
			if gotOk != tC.ok {
				t.Fatalf("%s failed: expected ok=%v got ok=%v notOkMsg=%s", tC.desc, tC.ok, gotOk, notOkMsg)
			}
			if gotOk && notOkMsg != "" {
				t.Fatalf("%s failed: expected got ok with non empty msg=%s", tC.desc, notOkMsg)
			}
			if !gotOk && notOkMsg == "" {
				t.Fatalf("%s failed: expected got notOk with empty msg", tC.desc)
			}
			if !gotOk {
				if tC.expectNotOkMsgToContain != "" {
					if !strings.Contains(notOkMsg, tC.expectNotOkMsgToContain) {
						t.Fatalf("%s failed: notOkMsg=%s doesnt contain %q",
							tC.desc, notOkMsg, tC.expectNotOkMsgToContain)
					}
				}
				return
			}
			if !reflect.DeepEqual(got, tC.expectedJobs) {
				t.Fatalf("%s failed: expected %#v got %#v", tC.desc, tC.expectedJobs, got)
			}
		})
	}
}