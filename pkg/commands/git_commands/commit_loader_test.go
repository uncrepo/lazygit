package git_commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-errors/errors"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/common"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/samber/lo"
	"github.com/stefanhaller/git-todo-parser/todo"
	"github.com/stretchr/testify/assert"
)

var commitsOutput = strings.ReplaceAll(`+0eea75e8c631fba6b58135697835d58ba4c18dbc|1640826609|Jesse Duffield|jessedduffield@gmail.com|HEAD -> better-tests|b21997d6b4cbdf84b149|>|better typing for rebase mode
+b21997d6b4cbdf84b149d8e6a2c4d06a8e9ec164|1640824515|Jesse Duffield|jessedduffield@gmail.com|origin/better-tests|e94e8fc5b6fab4cb755f|>|fix logging
+e94e8fc5b6fab4cb755f29f1bdb3ee5e001df35c|1640823749|Jesse Duffield|jessedduffield@gmail.com|tag: 123, tag: 456|d8084cd558925eb7c9c3|>|refactor
+d8084cd558925eb7c9c38afeed5725c21653ab90|1640821426|Jesse Duffield|jessedduffield@gmail.com||65f910ebd85283b5cce9|>|WIP
+65f910ebd85283b5cce9bf67d03d3f1a9ea3813a|1640821275|Jesse Duffield|jessedduffield@gmail.com||26c07b1ab33860a1a759|>|WIP
+26c07b1ab33860a1a7591a0638f9925ccf497ffa|1640750752|Jesse Duffield|jessedduffield@gmail.com||3d4470a6c072208722e5|>|WIP
+3d4470a6c072208722e5ae9a54bcb9634959a1c5|1640748818|Jesse Duffield|jessedduffield@gmail.com||053a66a7be3da43aacdc|>|WIP
+053a66a7be3da43aacdc7aa78e1fe757b82c4dd2|1640739815|Jesse Duffield|jessedduffield@gmail.com||985fe482e806b172aea4|>|refactoring the config struct`, "|", "\x00")

var singleCommitOutput = strings.ReplaceAll(`+0eea75e8c631fba6b58135697835d58ba4c18dbc|1640826609|Jesse Duffield|jessedduffield@gmail.com|HEAD -> better-tests|b21997d6b4cbdf84b149|>|better typing for rebase mode`, "|", "\x00")

func TestGetCommits(t *testing.T) {
	type scenario struct {
		testName           string
		runner             *oscommands.FakeCmdObjRunner
		expectedCommitOpts []models.NewCommitOpts
		expectedError      error
		logOrder           string
		opts               GetCommitsOptions
		mainBranches       []string
	}

	scenarios := []scenario{
		{
			testName: "should return no commits if there are none",
			logOrder: "topo-order",
			opts:     GetCommitsOptions{RefName: "HEAD", RefForPushedStatus: "mybranch", IncludeRebaseCommits: false},
			runner: oscommands.NewFakeRunner(t).
				ExpectGitArgs([]string{"merge-base", "mybranch", "mybranch@{u}"}, "b21997d6b4cbdf84b149d8e6a2c4d06a8e9ec164", nil).
				ExpectGitArgs([]string{"log", "HEAD", "--topo-order", "--oneline", "--pretty=format:+%H%x00%at%x00%aN%x00%ae%x00%D%x00%P%x00%m%x00%s", "--abbrev=40", "--no-show-signature", "--"}, "", nil),

			expectedCommitOpts: []models.NewCommitOpts{},
			expectedError:      nil,
		},
		{
			testName: "should use proper upstream name for branch",
			logOrder: "topo-order",
			opts:     GetCommitsOptions{RefName: "refs/heads/mybranch", RefForPushedStatus: "refs/heads/mybranch", IncludeRebaseCommits: false},
			runner: oscommands.NewFakeRunner(t).
				ExpectGitArgs([]string{"merge-base", "refs/heads/mybranch", "mybranch@{u}"}, "b21997d6b4cbdf84b149d8e6a2c4d06a8e9ec164", nil).
				ExpectGitArgs([]string{"log", "refs/heads/mybranch", "--topo-order", "--oneline", "--pretty=format:+%H%x00%at%x00%aN%x00%ae%x00%D%x00%P%x00%m%x00%s", "--abbrev=40", "--no-show-signature", "--"}, "", nil),

			expectedCommitOpts: []models.NewCommitOpts{},
			expectedError:      nil,
		},
		{
			testName:     "should return commits if they are present",
			logOrder:     "topo-order",
			opts:         GetCommitsOptions{RefName: "HEAD", RefForPushedStatus: "mybranch", IncludeRebaseCommits: false},
			mainBranches: []string{"master", "main", "develop"},
			runner: oscommands.NewFakeRunner(t).
				// here it's seeing which commits are yet to be pushed
				ExpectGitArgs([]string{"merge-base", "mybranch", "mybranch@{u}"}, "b21997d6b4cbdf84b149d8e6a2c4d06a8e9ec164", nil).
				// here it's actually getting all the commits in a formatted form, one per line
				ExpectGitArgs([]string{"log", "HEAD", "--topo-order", "--oneline", "--pretty=format:+%H%x00%at%x00%aN%x00%ae%x00%D%x00%P%x00%m%x00%s", "--abbrev=40", "--no-show-signature", "--"}, commitsOutput, nil).
				// here it's testing which of the configured main branches have an upstream
				ExpectGitArgs([]string{"rev-parse", "--symbolic-full-name", "master@{u}"}, "refs/remotes/origin/master", nil).       // this one does
				ExpectGitArgs([]string{"rev-parse", "--symbolic-full-name", "main@{u}"}, "", errors.New("error")).                   // this one doesn't, so it checks origin instead
				ExpectGitArgs([]string{"rev-parse", "--verify", "--quiet", "refs/remotes/origin/main"}, "", nil).                    // yep, origin/main exists
				ExpectGitArgs([]string{"rev-parse", "--symbolic-full-name", "develop@{u}"}, "", errors.New("error")).                // this one doesn't, so it checks origin instead
				ExpectGitArgs([]string{"rev-parse", "--verify", "--quiet", "refs/remotes/origin/develop"}, "", errors.New("error")). // doesn't exist there, either, so it checks for a local branch
				ExpectGitArgs([]string{"rev-parse", "--verify", "--quiet", "refs/heads/develop"}, "", errors.New("error")).          // no local branch either
				// here it's seeing where our branch diverged from the master branch so that we can mark that commit and parent commits as 'merged'
				ExpectGitArgs([]string{"merge-base", "HEAD", "refs/remotes/origin/master", "refs/remotes/origin/main"}, "26c07b1ab33860a1a7591a0638f9925ccf497ffa", nil),

			expectedCommitOpts: []models.NewCommitOpts{
				{
					Hash:          "0eea75e8c631fba6b58135697835d58ba4c18dbc",
					Name:          "better typing for rebase mode",
					Status:        models.StatusUnpushed,
					Action:        models.ActionNone,
					Tags:          []string{},
					ExtraInfo:     "(HEAD -> better-tests)",
					AuthorName:    "Jesse Duffield",
					AuthorEmail:   "jessedduffield@gmail.com",
					UnixTimestamp: 1640826609,
					Parents: []string{
						"b21997d6b4cbdf84b149",
					},
				},
				{
					Hash:          "b21997d6b4cbdf84b149d8e6a2c4d06a8e9ec164",
					Name:          "fix logging",
					Status:        models.StatusPushed,
					Action:        models.ActionNone,
					Tags:          []string{},
					ExtraInfo:     "(origin/better-tests)",
					AuthorName:    "Jesse Duffield",
					AuthorEmail:   "jessedduffield@gmail.com",
					UnixTimestamp: 1640824515,
					Parents: []string{
						"e94e8fc5b6fab4cb755f",
					},
				},
				{
					Hash:          "e94e8fc5b6fab4cb755f29f1bdb3ee5e001df35c",
					Name:          "refactor",
					Status:        models.StatusPushed,
					Action:        models.ActionNone,
					Tags:          []string{"123", "456"},
					ExtraInfo:     "(tag: 123, tag: 456)",
					AuthorName:    "Jesse Duffield",
					AuthorEmail:   "jessedduffield@gmail.com",
					UnixTimestamp: 1640823749,
					Parents: []string{
						"d8084cd558925eb7c9c3",
					},
				},
				{
					Hash:          "d8084cd558925eb7c9c38afeed5725c21653ab90",
					Name:          "WIP",
					Status:        models.StatusPushed,
					Action:        models.ActionNone,
					Tags:          []string{},
					ExtraInfo:     "",
					AuthorName:    "Jesse Duffield",
					AuthorEmail:   "jessedduffield@gmail.com",
					UnixTimestamp: 1640821426,
					Parents: []string{
						"65f910ebd85283b5cce9",
					},
				},
				{
					Hash:          "65f910ebd85283b5cce9bf67d03d3f1a9ea3813a",
					Name:          "WIP",
					Status:        models.StatusPushed,
					Action:        models.ActionNone,
					Tags:          []string{},
					ExtraInfo:     "",
					AuthorName:    "Jesse Duffield",
					AuthorEmail:   "jessedduffield@gmail.com",
					UnixTimestamp: 1640821275,
					Parents: []string{
						"26c07b1ab33860a1a759",
					},
				},
				{
					Hash:          "26c07b1ab33860a1a7591a0638f9925ccf497ffa",
					Name:          "WIP",
					Status:        models.StatusMerged,
					Action:        models.ActionNone,
					Tags:          []string{},
					ExtraInfo:     "",
					AuthorName:    "Jesse Duffield",
					AuthorEmail:   "jessedduffield@gmail.com",
					UnixTimestamp: 1640750752,
					Parents: []string{
						"3d4470a6c072208722e5",
					},
				},
				{
					Hash:          "3d4470a6c072208722e5ae9a54bcb9634959a1c5",
					Name:          "WIP",
					Status:        models.StatusMerged,
					Action:        models.ActionNone,
					Tags:          []string{},
					ExtraInfo:     "",
					AuthorName:    "Jesse Duffield",
					AuthorEmail:   "jessedduffield@gmail.com",
					UnixTimestamp: 1640748818,
					Parents: []string{
						"053a66a7be3da43aacdc",
					},
				},
				{
					Hash:          "053a66a7be3da43aacdc7aa78e1fe757b82c4dd2",
					Name:          "refactoring the config struct",
					Status:        models.StatusMerged,
					Action:        models.ActionNone,
					Tags:          []string{},
					ExtraInfo:     "",
					AuthorName:    "Jesse Duffield",
					AuthorEmail:   "jessedduffield@gmail.com",
					UnixTimestamp: 1640739815,
					Parents: []string{
						"985fe482e806b172aea4",
					},
				},
			},
			expectedError: nil,
		},
		{
			testName:     "should not call merge-base for mainBranches if none exist",
			logOrder:     "topo-order",
			opts:         GetCommitsOptions{RefName: "HEAD", RefForPushedStatus: "mybranch", IncludeRebaseCommits: false},
			mainBranches: []string{"master", "main"},
			runner: oscommands.NewFakeRunner(t).
				// here it's seeing which commits are yet to be pushed
				ExpectGitArgs([]string{"merge-base", "mybranch", "mybranch@{u}"}, "b21997d6b4cbdf84b149d8e6a2c4d06a8e9ec164", nil).
				// here it's actually getting all the commits in a formatted form, one per line
				ExpectGitArgs([]string{"log", "HEAD", "--topo-order", "--oneline", "--pretty=format:+%H%x00%at%x00%aN%x00%ae%x00%D%x00%P%x00%m%x00%s", "--abbrev=40", "--no-show-signature", "--"}, singleCommitOutput, nil).
				// here it's testing which of the configured main branches exist; neither does
				ExpectGitArgs([]string{"rev-parse", "--symbolic-full-name", "master@{u}"}, "", errors.New("error")).
				ExpectGitArgs([]string{"rev-parse", "--verify", "--quiet", "refs/remotes/origin/master"}, "", errors.New("error")).
				ExpectGitArgs([]string{"rev-parse", "--verify", "--quiet", "refs/heads/master"}, "", errors.New("error")).
				ExpectGitArgs([]string{"rev-parse", "--symbolic-full-name", "main@{u}"}, "", errors.New("error")).
				ExpectGitArgs([]string{"rev-parse", "--verify", "--quiet", "refs/remotes/origin/main"}, "", errors.New("error")).
				ExpectGitArgs([]string{"rev-parse", "--verify", "--quiet", "refs/heads/main"}, "", errors.New("error")),

			expectedCommitOpts: []models.NewCommitOpts{
				{
					Hash:          "0eea75e8c631fba6b58135697835d58ba4c18dbc",
					Name:          "better typing for rebase mode",
					Status:        models.StatusUnpushed,
					Action:        models.ActionNone,
					Tags:          []string{},
					ExtraInfo:     "(HEAD -> better-tests)",
					AuthorName:    "Jesse Duffield",
					AuthorEmail:   "jessedduffield@gmail.com",
					UnixTimestamp: 1640826609,
					Parents: []string{
						"b21997d6b4cbdf84b149",
					},
				},
			},
			expectedError: nil,
		},
		{
			testName:     "should call merge-base for all main branches that exist",
			logOrder:     "topo-order",
			opts:         GetCommitsOptions{RefName: "HEAD", RefForPushedStatus: "mybranch", IncludeRebaseCommits: false},
			mainBranches: []string{"master", "main", "develop", "1.0-hotfixes"},
			runner: oscommands.NewFakeRunner(t).
				// here it's seeing which commits are yet to be pushed
				ExpectGitArgs([]string{"merge-base", "mybranch", "mybranch@{u}"}, "b21997d6b4cbdf84b149d8e6a2c4d06a8e9ec164", nil).
				// here it's actually getting all the commits in a formatted form, one per line
				ExpectGitArgs([]string{"log", "HEAD", "--topo-order", "--oneline", "--pretty=format:+%H%x00%at%x00%aN%x00%ae%x00%D%x00%P%x00%m%x00%s", "--abbrev=40", "--no-show-signature", "--"}, singleCommitOutput, nil).
				// here it's testing which of the configured main branches exist
				ExpectGitArgs([]string{"rev-parse", "--symbolic-full-name", "master@{u}"}, "refs/remotes/origin/master", nil).
				ExpectGitArgs([]string{"rev-parse", "--symbolic-full-name", "main@{u}"}, "", errors.New("error")).
				ExpectGitArgs([]string{"rev-parse", "--verify", "--quiet", "refs/remotes/origin/main"}, "", errors.New("error")).
				ExpectGitArgs([]string{"rev-parse", "--verify", "--quiet", "refs/heads/main"}, "", errors.New("error")).
				ExpectGitArgs([]string{"rev-parse", "--symbolic-full-name", "develop@{u}"}, "refs/remotes/origin/develop", nil).
				ExpectGitArgs([]string{"rev-parse", "--symbolic-full-name", "1.0-hotfixes@{u}"}, "refs/remotes/origin/1.0-hotfixes", nil).
				// here it's seeing where our branch diverged from the master branch so that we can mark that commit and parent commits as 'merged'
				ExpectGitArgs([]string{"merge-base", "HEAD", "refs/remotes/origin/master", "refs/remotes/origin/develop", "refs/remotes/origin/1.0-hotfixes"}, "26c07b1ab33860a1a7591a0638f9925ccf497ffa", nil),

			expectedCommitOpts: []models.NewCommitOpts{
				{
					Hash:          "0eea75e8c631fba6b58135697835d58ba4c18dbc",
					Name:          "better typing for rebase mode",
					Status:        models.StatusUnpushed,
					Action:        models.ActionNone,
					Tags:          []string{},
					ExtraInfo:     "(HEAD -> better-tests)",
					AuthorName:    "Jesse Duffield",
					AuthorEmail:   "jessedduffield@gmail.com",
					UnixTimestamp: 1640826609,
					Parents: []string{
						"b21997d6b4cbdf84b149",
					},
				},
			},
			expectedError: nil,
		},
		{
			testName: "should not specify order if `log.order` is `default`",
			logOrder: "default",
			opts:     GetCommitsOptions{RefName: "HEAD", RefForPushedStatus: "mybranch", IncludeRebaseCommits: false},
			runner: oscommands.NewFakeRunner(t).
				ExpectGitArgs([]string{"merge-base", "mybranch", "mybranch@{u}"}, "b21997d6b4cbdf84b149d8e6a2c4d06a8e9ec164", nil).
				ExpectGitArgs([]string{"log", "HEAD", "--oneline", "--pretty=format:+%H%x00%at%x00%aN%x00%ae%x00%D%x00%P%x00%m%x00%s", "--abbrev=40", "--no-show-signature", "--"}, "", nil),

			expectedCommitOpts: []models.NewCommitOpts{},
			expectedError:      nil,
		},
		{
			testName: "should set filter path",
			logOrder: "default",
			opts:     GetCommitsOptions{RefName: "HEAD", RefForPushedStatus: "mybranch", FilterPath: "src"},
			runner: oscommands.NewFakeRunner(t).
				ExpectGitArgs([]string{"merge-base", "mybranch", "mybranch@{u}"}, "b21997d6b4cbdf84b149d8e6a2c4d06a8e9ec164", nil).
				ExpectGitArgs([]string{"log", "HEAD", "--oneline", "--pretty=format:+%H%x00%at%x00%aN%x00%ae%x00%D%x00%P%x00%m%x00%s", "--abbrev=40", "--follow", "--name-status", "--no-show-signature", "--", "src"}, "", nil),

			expectedCommitOpts: []models.NewCommitOpts{},
			expectedError:      nil,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.testName, func(t *testing.T) {
			common := common.NewDummyCommon()
			common.UserConfig().Git.Log.Order = scenario.logOrder
			cmd := oscommands.NewDummyCmdObjBuilder(scenario.runner)

			builder := &CommitLoader{
				Common:              common,
				cmd:                 cmd,
				getWorkingTreeState: func() models.WorkingTreeState { return models.WorkingTreeState{} },
				dotGitDir:           ".git",
				readFile: func(filename string) ([]byte, error) {
					return []byte(""), nil
				},
				walkFiles: func(root string, fn filepath.WalkFunc) error {
					return nil
				},
			}

			hashPool := &utils.StringPool{}

			common.UserConfig().Git.MainBranches = scenario.mainBranches
			opts := scenario.opts
			opts.MainBranches = NewMainBranches(common, cmd)
			opts.HashPool = hashPool
			commits, err := builder.GetCommits(opts)

			expectedCommits := lo.Map(scenario.expectedCommitOpts,
				func(opts models.NewCommitOpts, _ int) *models.Commit { return models.NewCommit(hashPool, opts) })
			assert.Equal(t, expectedCommits, commits)
			assert.Equal(t, scenario.expectedError, err)

			scenario.runner.CheckForMissingCalls()
		})
	}
}

func TestCommitLoader_getConflictedCommitImpl(t *testing.T) {
	hashPool := &utils.StringPool{}

	scenarios := []struct {
		testName          string
		todos             []todo.Todo
		doneTodos         []todo.Todo
		amendFileExists   bool
		messageFileExists bool
		expectedResult    *models.Commit
	}{
		{
			testName:        "no done todos",
			todos:           []todo.Todo{},
			doneTodos:       []todo.Todo{},
			amendFileExists: false,
			expectedResult:  nil,
		},
		{
			testName: "common case (conflict)",
			todos:    []todo.Todo{},
			doneTodos: []todo.Todo{
				{
					Command: todo.Pick,
					Commit:  "deadbeef",
				},
				{
					Command: todo.Pick,
					Commit:  "fa1afe1",
				},
			},
			amendFileExists: false,
			expectedResult: models.NewCommit(hashPool, models.NewCommitOpts{
				Hash:   "fa1afe1",
				Action: todo.Pick,
				Status: models.StatusConflicted,
			}),
		},
		{
			testName: "last command was 'break'",
			todos:    []todo.Todo{},
			doneTodos: []todo.Todo{
				{Command: todo.Break},
			},
			amendFileExists: false,
			expectedResult:  nil,
		},
		{
			testName: "last command was 'exec'",
			todos:    []todo.Todo{},
			doneTodos: []todo.Todo{
				{
					Command:     todo.Exec,
					ExecCommand: "make test",
				},
			},
			amendFileExists: false,
			expectedResult:  nil,
		},
		{
			testName: "last command was 'reword'",
			todos:    []todo.Todo{},
			doneTodos: []todo.Todo{
				{Command: todo.Reword},
			},
			amendFileExists: false,
			expectedResult:  nil,
		},
		{
			testName: "'pick' was rescheduled",
			todos: []todo.Todo{
				{
					Command: todo.Pick,
					Commit:  "fa1afe1",
				},
			},
			doneTodos: []todo.Todo{
				{
					Command: todo.Pick,
					Commit:  "fa1afe1",
				},
			},
			amendFileExists: false,
			expectedResult:  nil,
		},
		{
			testName: "'pick' was rescheduled, buggy git version",
			todos: []todo.Todo{
				{
					Command: todo.Pick,
					Commit:  "fa1afe1",
				},
			},
			doneTodos: []todo.Todo{
				{
					Command: todo.Pick,
					Commit:  "deadbeaf",
				},
				{
					Command: todo.Pick,
					Commit:  "fa1afe1",
				},
				{
					Command: todo.Pick,
					Commit:  "deadbeaf",
				},
			},
			amendFileExists: false,
			expectedResult:  nil,
		},
		{
			testName: "conflicting 'pick' after 'exec'",
			todos: []todo.Todo{
				{
					Command:     todo.Exec,
					ExecCommand: "make test",
				},
			},
			doneTodos: []todo.Todo{
				{
					Command: todo.Pick,
					Commit:  "deadbeaf",
				},
				{
					Command:     todo.Exec,
					ExecCommand: "make test",
				},
				{
					Command: todo.Pick,
					Commit:  "fa1afe1",
				},
			},
			amendFileExists: false,
			expectedResult: models.NewCommit(hashPool, models.NewCommitOpts{
				Hash:   "fa1afe1",
				Action: todo.Pick,
				Status: models.StatusConflicted,
			}),
		},
		{
			testName: "'edit' with amend file",
			todos:    []todo.Todo{},
			doneTodos: []todo.Todo{
				{
					Command: todo.Edit,
					Commit:  "fa1afe1",
				},
			},
			amendFileExists: true,
			expectedResult:  nil,
		},
		{
			testName: "'edit' without amend file but message file",
			todos:    []todo.Todo{},
			doneTodos: []todo.Todo{
				{
					Command: todo.Edit,
					Commit:  "fa1afe1",
				},
			},
			amendFileExists:   false,
			messageFileExists: true,
			expectedResult: models.NewCommit(hashPool, models.NewCommitOpts{
				Hash:   "fa1afe1",
				Action: todo.Edit,
				Status: models.StatusConflicted,
			}),
		},
		{
			testName: "'edit' without amend and without message file",
			todos:    []todo.Todo{},
			doneTodos: []todo.Todo{
				{
					Command: todo.Edit,
					Commit:  "fa1afe1",
				},
			},
			amendFileExists:   false,
			messageFileExists: false,
			expectedResult:    nil,
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.testName, func(t *testing.T) {
			common := common.NewDummyCommon()

			builder := &CommitLoader{
				Common:              common,
				cmd:                 oscommands.NewDummyCmdObjBuilder(oscommands.NewFakeRunner(t)),
				getWorkingTreeState: func() models.WorkingTreeState { return models.WorkingTreeState{Rebasing: true} },
				dotGitDir:           ".git",
				readFile: func(filename string) ([]byte, error) {
					return []byte(""), nil
				},
				walkFiles: func(root string, fn filepath.WalkFunc) error {
					return nil
				},
			}

			hash := builder.getConflictedCommitImpl(hashPool, scenario.todos, scenario.doneTodos, scenario.amendFileExists, scenario.messageFileExists)
			assert.Equal(t, scenario.expectedResult, hash)
		})
	}
}

func TestCommitLoader_setCommitMergedStatuses(t *testing.T) {
	type scenario struct {
		testName           string
		commitOpts         []models.NewCommitOpts
		ancestor           string
		expectedCommitOpts []models.NewCommitOpts
	}

	scenarios := []scenario{
		{
			testName: "basic",
			commitOpts: []models.NewCommitOpts{
				{Hash: "12345", Name: "1", Action: models.ActionNone, Status: models.StatusUnpushed},
				{Hash: "67890", Name: "2", Action: models.ActionNone, Status: models.StatusPushed},
				{Hash: "abcde", Name: "3", Action: models.ActionNone, Status: models.StatusPushed},
			},
			ancestor: "67890",
			expectedCommitOpts: []models.NewCommitOpts{
				{Hash: "12345", Name: "1", Action: models.ActionNone, Status: models.StatusUnpushed},
				{Hash: "67890", Name: "2", Action: models.ActionNone, Status: models.StatusMerged},
				{Hash: "abcde", Name: "3", Action: models.ActionNone, Status: models.StatusMerged},
			},
		},
		{
			testName: "with update-ref",
			commitOpts: []models.NewCommitOpts{
				{Hash: "12345", Name: "1", Action: models.ActionNone, Status: models.StatusUnpushed},
				{Hash: "", Name: "", Action: todo.UpdateRef, Status: models.StatusNone},
				{Hash: "abcde", Name: "3", Action: models.ActionNone, Status: models.StatusPushed},
			},
			ancestor: "deadbeef",
			expectedCommitOpts: []models.NewCommitOpts{
				{Hash: "12345", Name: "1", Action: models.ActionNone, Status: models.StatusUnpushed},
				{Hash: "", Name: "", Action: todo.UpdateRef, Status: models.StatusNone},
				{Hash: "abcde", Name: "3", Action: models.ActionNone, Status: models.StatusPushed},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.testName, func(t *testing.T) {
			hashPool := &utils.StringPool{}

			commits := lo.Map(scenario.commitOpts,
				func(opts models.NewCommitOpts, _ int) *models.Commit { return models.NewCommit(hashPool, opts) })
			setCommitMergedStatuses(scenario.ancestor, commits)
			expectedCommits := lo.Map(scenario.expectedCommitOpts,
				func(opts models.NewCommitOpts, _ int) *models.Commit { return models.NewCommit(hashPool, opts) })
			assert.Equal(t, expectedCommits, commits)
		})
	}
}
