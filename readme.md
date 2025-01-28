![logo](docs/git_spr_logo.png)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build](https://github.com/ejoffe/spr/actions/workflows/ci.yml/badge.svg)](https://github.com/ejoffe/spr/actions/workflows/ci.yml)
[![ReportCard](https://goreportcard.com/badge/github.com/ejoffe/spr)](https://goreportcard.com/report/github.com/ejoffe/spr)
[![Doc](https://godoc.org/github.com/ejoffe/spr?status.svg)](https://godoc.org/github.com/ejoffe/spr)
[![Release](https://img.shields.io/github/release/ejoffe/spr.svg)](https://GitHub.com/ejoffe/spr/releases/) 
[![Join the chat at https://gitter.im/ejoffe-spr/community](https://badges.gitter.im/ejoffe-spr/community.svg)](https://gitter.im/ejoffe-spr/community?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

![terminal cast](docs/git_spr_cast.gif)
## GraphQL Users
Checkout my company [Inigo](https://www.inigo.io) for the best holistic platform for your api.

# Stacked Pull Requests on GitHub

Easily manage stacks of pull requests on GitHub. 
`git spr` is a client side tool that achieves a simple streamlined stacked diff workflow using github pull requests and branches. `git spr` manages your pull request stacks for you, so you don't have to. 

With `git spr` each commit becomes a pull request, and each branch becomes a stack of pull requests. This allows for multiple commits to be stacked on top of each other in a single branch, avoiding the overhead of starting a new branch for every new change or feature. Small changes and pull requests are easy and fast to achieve. One doesn't have to worry about stacked branches on top of each other and managing complicated pull request stacks. The end result is a more streamlined faster software development cycle.

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

To update only part of the stack use the `--count` flag with the number of pull requests in the stack that you would like to update. Pull requests will be updated from the bottom of the stack upwards. 

Amending Commits
----------------
When you need to update a commit, either to fix tests, update code based on review comments, or just need to change something because you feel like it. You should amend the commit. 
Use `git amend` to easily amend your changes anywhere in the stack. Stage the files you want to amend, and instead of calling git commit, use `git amend` and choose the commit you want to amend when prompted.  
```shell
> touch feature_2
> git add feature_2
> git amend
 3 : 5cba235d : Feature 3
 2 : 4dc2c5b2 : Feature 2
 1 : 9d1b8193 : Feature 1
Commit to amend [1-3]: 2
```

Merge Status Bits
-----------------
Each pull request has four merge status bits signifying the request's ability to be merged. For a request to be merged, all required status bits need to show **✔**. Each status bit has the following meaning:
1. github checks run and pass 
  - ⌛ : pending
  - ❌ : some check failed
  - ✅ : all checks pass
  - ➖ : checks are not required to merge (can be configured in yml config)
2. pull request approval
  - ❌ : pull request hasn't been approved
  - ✅ : pull request is approved
  - ➖ : approval is not required to merge (can be configured in yml config)
3. merge conflicts
  - ❌ : commit has conflicts that need to be resolved
  - ✅ : commit has no conflicts
4. stack status
  - ❌ : commit has other pull requests below it that can't merge
  - ✅ : all commits below this one are clear to merge

Pull request approval and checks requirement can be disabled in the config file, see configuration section below for more details.

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

By default merges are done using the rebase merge method, this can be changed using the mergeMethod configuration.

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
| prTemplatePath          | str  |            | path to PR template (e.g. .github/PULL_REQUEST_TEMPLATE/pull_request_template.md) |
| prTemplateInsertStart   | str  |            | text to search for in PR template that determines body insert start location |
| prTemplateInsertEnd     | str  |            | text to search for in PR template that determines body insert end location |
| mergeCheck              | str  |            | enforce a pre-merge check using 'git spr check' |
| forceFetchTags          | bool | false      | also fetch tags when running 'git spr update' |
| branchNameIncludeTarget | bool | false      | include target branch name in pull request branch name |
| showPrTitlesInStack     | bool | false      | show PR titles in stack description within pull request body |
| branchPushIndividually  | bool | false      | push branches individually instead of atomically (only enable to avoid timeouts) |


| User Config          | Type | Default | Description                                                     |
| -------------------- | ---- | ------- | --------------------------------------------------------------- |
| showPRLink           | bool | true    | show full pull request http link |
| logGitCommands       | bool | true    | logs all git commands to stdout |
| logGitHubCalls       | bool | true    | logs all github api calls to stdout |
| statusBitsHeader     | bool | true    | show status bits type headers |
| statusBitsEmojis     | bool | true    | show status bits using fancy emojis |
| createDraftPRs       | bool | false   | new pull requests are created as draft |
| preserveTitleAndBody | bool | false   | updating pull requests will not overwrite the pr title and body |
| noRebase             | bool | false   | when true spr update will not rebase on top of origin |
| deleteMergedBranches | bool | false   | delete branches after prs are merged |

Happy Coding!
-------------
If you find a bug, feel free to open an issue. Pull requests are welcome.

If you find this script as useful as I do, add a **star** and tell your fellow githubers.

License
-------

- [MIT License](LICENSE)
