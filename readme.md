![logo](docs/git_spr_logo.png)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build](https://github.com/ejoffe/spr/actions/workflows/ci.yml/badge.svg)](https://github.com/ejoffe/spr/actions/workflows/ci.yml)
[![ReportCard](https://goreportcard.com/badge/github.com/ejoffe/spr)](https://goreportcard.com/report/github.com/ejoffe/spr)
[![Doc](https://godoc.org/github.com/ejoffe/spr?status.svg)](https://godoc.org/github.com/ejoffe/spr)
[![Release](https://img.shields.io/github/release/ejoffe/spr.svg)](https://GitHub.com/ejoffe/spr/releases/) 
[![Join the chat at https://gitter.im/ejoffe-spr/community](https://badges.gitter.im/ejoffe-spr/community.svg)](https://gitter.im/ejoffe-spr/community?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

![terminal cast](docs/git_spr_cast.gif)

# Stacked Pull Requests on GitHub

Easily manage stacks of pull requests on GitHub. 
`git spr` is a client side tool that achieves a simple streamlined stacked diff workflow using github pull requests and branches. `git spr` manages your pull request stacks for you, so you don't have to. 

With `git spr` each commit becomes a pull request, and each branch becomes a stack of pull requests. This allows for multiple commits to be stacked on top of each other in a single branch, avoiding the overhead of starting a new branch for every new change or feature. Small changes and pull requests are easy and fast to achieve. One doesn't have to worry about stacked branches on top of each other and managing complicated pull request stacks. The end result is a more streamlined faster software development cycle.

Commands
--------

| Command | Aliases | Description |
|---------|---------|-------------|
| `git spr update`  | `u`, `up` | Create and update pull requests for commits in the stack |
| `git spr status`  | `s`, `st` | Show status of open pull requests |
| `git spr merge`   |           | Merge all mergeable pull requests |
| `git spr amend`   | `a`       | Amend a commit in the stack |
| `git spr edit`    | `e`       | Edit a commit in the stack (interactive rebase) |
| `git spr sync`    |           | Synchronize local stack with remote |
| `git spr check`   |           | Run pre-merge checks (configured by `mergeCheck`) |
| `git spr version` |           | Show version info |

**Global flags:** `--detail` (show status bit headers), `--verbose` (log git commands and GitHub API calls), `--no-jj` (disable jj mode), `--debug`, `--profile`

Installation 
------------

### Brew
```shell
brew tap ejoffe/homebrew-tap
brew install ejoffe/tap/spr
```

### Apt
```shell
echo "deb [trusted=yes] https://apt.fury.io/inigolabs/ /" | sudo tee /etc/apt/sources.list.d/inigolabs.list
sudo apt update 
sudo apt install spr
```

### Nix
```shell
nix profile install github:ejoffe/spr
```
Or run without installing:
```shell
nix run github:ejoffe/spr
```

### Manual
Download the pre-compiled binaries from the [releases page](https://github.com/ejoffe/spr/releases) and copy to your bin path.

### From source
Install [goreleaser](https://goreleaser.com/) and run make. Binaries can be found in the **dist** directory.
```shell
make bin
```

Workflow
--------
Commit your changes to a branch as you normally do. Note that every commit will end up becoming a pull request.
```shell
> touch feature_1
> git add feature_1
> git commit -m "Feature 1"
> touch feature_2
> git add feature_2
> git commit -m "Feature 2"
> touch feature_3
> git add feature_3
> git commit -m "Feature 3"
```

The subject of the commit message will be the title of the pull request, and the body of the message will be the body of the pull request.
If you have a work in progress change that you want to commit, but don't want to create a pull request yet, start the commit message with all caps **WIP**. The spr script will not create a pull request for any commit which starts with WIP, when you are ready to create a pull request remove the WIP.
There is no need to create new branches for every change, and you don't have to call git push to get your code to github. Instead just call `git spr update`.

Managing Pull Requests
----------------------
Run `git spr update` to sync your whole commit stack to github and create pull requests for each new commit in the stack. If a commit was amended the pull request will be updated automatically. The command outputs a list of your open pull requests and their status. `git spr update` pushes your commits to github and creates pull requests for you, so you don't need to call git push or open pull requests manually in the UI.

```shell
> git spr update
[⌛❌✅❌] 60: Feature 3
[✅✅✅✅] 59: Feature 2
[✅✅✅✅] 58: Feature 1
```

| Flag | Alias | Description |
|------|-------|-------------|
| `--count`     | `-c` | Update a specific number of PRs from the bottom of the stack |
| `--reviewer`  | `-r` | Add reviewers to newly created pull requests |
| `--no-rebase` | `--nr` | Disable rebasing (also supports `SPR_NOREBASE` env var) |

Amending Commits
----------------
When you need to update a commit, either to fix tests, update code based on review comments, or just need to change something because you feel like it. You should amend the commit. 
Use `git amend` to easily amend your changes anywhere in the stack. Stage the files you want to amend, and instead of calling git commit, use `git amend` and choose the commit you want to amend when prompted.  
```shell
> touch feature_2
> git add feature_2
> git spr amend
 3 : 5cba235d : Feature 3
 2 : 4dc2c5b2 : Feature 2
 1 : 9d1b8193 : Feature 1
Commit to amend [1-3]: 2
```

Use `--update` (`-u`) to automatically run `git spr update` after amending.

Editing Commits
---------------
Use `git spr edit` to interactively edit a commit in the stack. This starts an interactive rebase session where you can make changes to the selected commit.

```shell
> git spr edit
 3 : 5cba235d : Feature 3
 2 : 4dc2c5b2 : Feature 2
 1 : 9d1b8193 : Feature 1
Commit to edit [1-3]: 2
```

Once you've made your changes, finish the session with `git spr edit --done`. Use `--update` (`-u`) with `--done` to also run `git spr update` afterwards. If you need to cancel, use `git spr edit --abort`.

Syncing Your Stack
------------------
Use `git spr sync` to synchronize your local stack with the remote. This is useful when pull requests have been updated on GitHub (e.g., after a merge) and you need to bring your local commits in line with the remote state.

Merge Status Bits
-----------------
Each pull request has four merge status bits signifying the request's ability to be merged. For a request to be merged, all required status bits need to show ✅.

| Bit | ⌛ | ❌ | ✅ | ➖ |
|-----|---|---|---|---|
| 1. Checks  | pending   | failed         | pass         | not required |
| 2. Approval | —        | not approved   | approved     | not required |
| 3. Conflicts | —      | has conflicts  | no conflicts | —           |
| 4. Stack   | —         | blocked below  | all clear    | —           |

Checks and approval requirements can be configured via `requireChecks` and `requireApproval` in `.spr.yml`.

Show Current Pull Requests
--------------------------
Use `git spr status` to see the status of your pull request stack. In the following case three pull requests are all green and ready to be merged, and one pull request is waiting for review approval. 

```shell
> git spr status
[✅❌✅✅] 61: Feature 4
[✅✅✅✅] 60: Feature 3
[✅✅✅✅] 59: Feature 2
[✅✅✅✅] 58: Feature 1
```

Merging Pull Requests
---------------------
Your pull requests are stacked. Don't use the GitHub UI to merge pull requests, if you do it in the wrong order, you'll end up pushing one pull request into another, which is probably not what you want. Instead just use `git spr merge` and you can merge all the pull requests that are mergeable in one shot. Status for the remaining pull requests will be printed after the merged requests.
In order to merge all pull requests in one shot without causing extra github checks to trigger, spr finds the top mergeable pull request. It then combines all the commits up to this pull request into one single pull request, merges this request, and closes the rest of the pull requests. This is a bit surprising at first, and has some side effects, but no better solution has been found to date. 

```shell
> git spr merge
MERGED #58 Feature 1
MERGED #59 Feature 2
MERGED #60 Feature 3
[✅❌✅✅] 61: Feature 4
```

To merge only part of the stack use the `--count` flag with the number of pull requests in the stack that you would like to merge. Pull requests will be merged from the bottom of the stack upwards. 

```shell
> git spr merge --count 2
MERGED #58 Feature 1
MERGED #59 Feature 2
[✅❌✅✅] 61: Feature 4
[✅✅✅✅] 60: Feature 3
```

By default merges are done using the rebase merge method, this can be changed using the `mergeMethod` configuration.

Running Pre-Merge Checks
-------------------------
Use `git spr check` to run a pre-merge check command configured via the `mergeCheck` repository setting. This lets you enforce that tests or linters pass before merging.

Starting a New Stack
---------------------
Starting a new stack works by creating a new branch. For example, if you want to start a new stack from the latest pushed state of your current branch, use `git checkout -b new_branch @{push}`.

Configuration
-------------
When the script is run for the first time two config files are created.
Repository configuration is saved to .spr.yml in the repository base directory. 
User specific configuration is saved to .spr.yml in the user home directory.

| Repository Config       | Type | Default    | Description                                                                       |
|-------------------------| ---- |------------|-----------------------------------------------------------------------------------|
| requireChecks           | bool | true       | require checks to pass in order to merge |
| requireApproval         | bool | true       | require pull request approval in order to merge |
| githubRepoOwner         | str  |            | name of the github owner (fetched from git remote config) |
| githubRepoName          | str  |            | name of the github repository (fetched from git remote config) |
| githubRemote            | str  | origin     | github remote name to use |
| githubBranch            | str  | main       | github branch for pull request target |
| githubHost              | str  | github.com | github host, can be updated for github enterprise use case |
| mergeMethod             | str  | rebase     | merge method, valid values: [rebase, squash, merge] |
| mergeQueue              | bool | false      | use GitHub merge queue to merge pull requests |
| prTemplateType          | str  | stack      | PR template type, valid values: [stack, basic, why_what, custom]. If prTemplatePath is provided, this is automatically set to "custom" |
| prTemplatePath          | str  |            | path to PR template file (e.g. .github/PULL_REQUEST_TEMPLATE/pull_request_template.md). When provided, prTemplateType is automatically set to "custom" |
| prTemplateInsertStart   | str  |            | text marker in PR template that determines where to insert commit body (used with custom template type) |
| prTemplateInsertEnd     | str  |            | text marker in PR template that determines where to end commit body insertion (used with custom template type) |
| mergeCheck              | str  |            | enforce a pre-merge check using 'git spr check' |
| forceFetchTags          | bool | false      | also fetch tags when running 'git spr update' |
| showPrTitlesInStack     | bool | false      | show PR titles in stack description within pull request body |
| branchPushIndividually  | bool | false      | push branches individually instead of atomically (only enable to avoid timeouts) |
| defaultReviewers        | list |            | default reviewers to add to each pull request |

| User Config          | Type | Default | Description                                                     |
| -------------------- | ---- | ------- | --------------------------------------------------------------- |
| showPRLink           | bool | true    | show full pull request http link |
| logGitCommands       | bool | false   | log git commands to stdout (enabled by `--verbose`) |
| logGitHubCalls       | bool | false   | log GitHub API calls to stdout (enabled by `--verbose`) |
| statusBitsHeader     | bool | true    | show status bits type headers |
| statusBitsEmojis     | bool | true    | show status bits using fancy emojis |
| createDraftPRs       | bool | false   | new pull requests are created as draft |
| preserveTitleAndBody | bool | false   | updating pull requests will not overwrite the pr title and body |
| noRebase             | bool | false   | when true spr update will not rebase on top of origin |
| noJJ                 | bool | false   | disable jj (Jujutsu) mode even in jj-colocated repos (also `--no-jj` flag or `SPR_NOJJ` env var) |
| deleteMergedBranches | bool | false   | delete branches after prs are merged |
| shortPRLink          | bool | false   | show pull request links as clickable PR-<number> instead of full URL |
| showCommitID         | bool | false   | show first 8 characters of commit hash for each pull request |

Jujutsu (jj) Support
--------------------
spr supports [Jujutsu](https://jj-vcs.github.io/jj/) colocated repositories. When spr detects a `.jj/` directory in your repo root, it automatically uses jj-native commands for history-rewriting operations (`jj describe`, `jj rebase`, `jj squash`, `jj edit`) instead of `git rebase`. This preserves jj change IDs across all spr operations.

Everything else (push, branch management, GitHub API calls) continues to use git, which works identically in colocated repos.

**Setup:**
```shell
# 1. Initialize jj in your existing git repo (if not already)
jj git init --colocate

# 2. Register the jj alias so you can use "jj spr" instead of "git spr"
git spr jj-setup
```

The `jj-setup` command adds a jj alias so spr can be invoked as `jj spr`:
```shell
jj spr update          # create/update PRs
jj spr status          # show PR status
jj spr merge           # merge PRs
jj spr amend           # amend a commit in the stack
```

You can also set up the alias manually:
```shell
jj config set --user aliases.spr '["util", "exec", "--", "git-spr"]'
```

**Opt-out:** If you have a `.jj/` directory but want spr to use git mode:
- CLI flag: `jj spr update --no-jj`
- Environment variable: `SPR_NOJJ=true`
- Config: add `noJJ: true` to `~/.spr.yml`

Happy Coding!
-------------
If you find a bug, feel free to open an issue. Pull requests are welcome.

If you find this tool as useful as I do, add a **star** and tell your fellow GitHubbers.

License
-------

- [MIT License](LICENSE)
