# git spr : Stacked Pull Requests on GitHub
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build](https://github.com/ejoffe/spr/actions/workflows/go_build.yml/badge.svg)](https://github.com/ejoffe/spr/actions/workflows/go_build.yml)
[![ReportCard](https://goreportcard.com/badge/github.com/ejoffe/spr)](https://goreportcard.com/report/github.com/ejoffe/spr)
[![Doc](https://godoc.org/github.com/ejoffe/spr?status.svg)](https://godoc.org/github.com/ejoffe/spr)
[![Release](https://img.shields.io/github/release/ejoffe/spr.svg)](https://GitHub.com/ejoffe/spr/releases/)

What is Stacked Diff Workflow?
------------------------------
Long explanation: https://jg.gg/2018/09/29/stacked-diffs-versus-pull-requests/

Short explanation: A git flow where the atomic unit of change is a commit. Each commit runs all the tests of ci and gets reviewed. When working with stacked diffs or commits, to add more stuff to a particular commit, you amend that commit. This allows for multiple commits to be stacked on top of each other in a single branch, avoiding the overhead of starting a new branch for every new change or feature. Small changes and pull requests are easy and fast to achieve. One doesn't have to worry about stacked branches on top of each other and managing complicated pull request stacks. The end result is a more streamlined faster software development cycle.

Stacked Pull Requests on GitHub
-------------------------------
    
The github flow is not quite compatible with the stacked diff model. At the core the atomic unit of change is a pull request which is based on a branch and can have many commits. Stacking pull requests on top of each other in github introduces extra overhead of managing all the commits in each branch of the pull request stack.
**git spr** is a client side tool that achieves a simple streamlined stacked diff workflow using github pull requests and branches. **git spr** manages your pull request stack for you, so you don't have to. 

Installation 
------------

### Brew
```bash
> brew tap ejoffe/homebrew-tap
> brew install spr
```

### DEB | RPM | APK
Download the .deb, .rpm or .apk from the [releases page](https://github.com/ejoffe/spr/releases) and install them with the appropriate tools.

### Manual
Download the pre-compiled binaries from the [releases page](https://github.com/ejoffe/spr/releases) and copy to the desired location.

Install Commit Hook
-------------------

A commit hook is used to add a commit-id tag to each commit. To install the commit hook, simply run the following in your root git directory.

```base
> ln -s $(which spr_commit_hook) .git/hooks/commit-msg
```

Workflow
--------
Create a branch to work in, or if you're brave enough, you can just work in the master branch.
```shell
> git branch -c salmon
```

In the stacked diff workflow you don't need to create a branch for every pull request or thing you want to do. You will generally work in one branch, but you can still create multiple branches and separate features into different branches if you want. Each branch will have it's own separate stack of pull requests. 
    
Commit your changes to the branch. Note that every commit will end up becoming a pull request.
```shell
> git add ..... 
> git commit  
```

The subject of the commit message will be the title of the pull request, and the body of the message will be the body of the pull request.
If you have a work in progress change that you want to commit, but don't want to create a pull request yet, start the commit message with all caps **WIP**. The spr script will not create a pull request for any commit which starts with WIP, when you are ready to create a pull request remove the WIP.

Managing Pull Requests
----------------------
Run **git spr update** to sync your whole commit stack to github and create pull requests for each new commit in the stack. If a commit was amended the pull request will be updated automatically. The command outputs a list of your open pull requests and their status. **git spr update** pushes your commits to github and creates pull requests for you, so you don't need to call git push or open pull requests manually in the UI.

```shell
> git spr update
[·✗✔✗] 61: Feature D
[·✗✔✗] 60: Feature C
[✔✔✔✔] 59: Feature B
[✔✔✔✔] 58: Feature A
```

Amending Commits
----------------
When you need to update a commit, either to fix tests, update code based on review comments, or just need to change something because you feel like it. You should amend the commit. 
Use **git amend** to easily amend your changes anywhere in the stack. Stage the files you want to amend, and instead of calling git commit, use the amend script and choose the commit you want to amend when prompted.  
```shell
> git add .....
> git amend
 3 : 5cba235d : Feature C
 2 : 4dc2c5b2 : Feature B
 1 : 9d1b8193 : Feature A
Commit to amend [1-3]: 2
```

If the commit is on top of the stack you can also simply:
```shell
> git add .....
> git commit --amend
```

Another approach is to create a new fixup commit on top of the stack and then use rebase to squash it into the right commit in the middle of the stack. 
```shell
> git add .....
> git commit -m "fixup to commit XXX"
> git rebase -i
```
Now use the editor to get the commit to the right place. **git rebase -i** is your friend, if you want to learn more about it: https://thoughtbot.com/blog/git-interactive-rebase-squash-amend-rewriting-history

Merge Status Bits
-----------------
Each pull request has four merge status bits signifying the request's ability to be merged. For a request to be merged, all required status bits need to show **✔**. Each status bit has the following meaning:
1. github checks run and pass 
  - · : pending 
  - ✗ : some check failed 
  - ✔ : all checks pass 
  - \- : checks are not required to merge (can be configured in yml config)
2. pull request approval
  - ✗ : pull request hasn't been approved
  - ✔ : pull request is approved
  - \- : approval is not required to merge (can be configured in yml config)
3. merge conflicts
  - ✗ : commit has conflicts that need to be resolved
  - ✔ : commit has no conflicts 
4. stack status
  - ✗ : commit has other pull requests below it that can't merge
  - ✔ : all commits below this one are clear to merge

Merging Pull Requests
---------------------
Your pull requests are stacked. Don't use the UI to merge pull requests, if you do it in the wrong order, you'll end up pushing one pull request into another, which is probably not what you want. Instead just use **git spr merge** and you can merge all the pull requests that are mergeable in one shot. Status for the remaining pull requests will be printed after the merged requests.

```shell
> git spr merge
MERGED #58 Feature A
MERGED #59 Feature B
[·✗✔✗] 61: Feature D
[·✗✔✗] 60: Feature C
```

Show Current Pull Requests
--------------------------
```shell
> git spr
[·✗✔✗] 61: Feature D
[·✗✔✗] 60: Feature C
```

Configuration
-------------
When the script is run for the first time a config file **.spr.yml** is created in your repository base dir. 
These are the available configurations:

| Name                | Type | Default | Description
| ------------------- | ---- | ------- | -------------------------------------------------------------- |
| githubRepoOwner     | str  |         | name of the github owner (fetched from git remote config)      |
| githubRepoName      | str  |         | name of the github repository (fetched from git remote config) |
| requireChecks       | bool | true    | require checks to pass in order to merge                       |
| requireApproval     | bool | true    | require pull request approval in order to merge                |
| showPRLink          | bool | true    | show full pull request http link                               |
| cleanupRemoteBranch | bool | true    | delete remote branch after pull request merges                 |
| logGitCommands      | bool | false   | logs all git commands to stdout                                |
| logGitHubCalls      | bool | false   | logs all github api calls to stdout                            |
| statusBitsHeader    | bool | true    | show status bits type headers                                  |

Happy Coding!
-------------
If you find a bug, feel free to open an issue. Pull requests are welcome.

If you find this script as useful as I do, add a **star** and tell your fellow githubers.

License
-------

- [MIT License](LICENSE)
