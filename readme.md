# Stacked Diff Workflow for GitHub

## What are Stacked Diffs?
Long explanation: https://jg.gg/2018/09/29/stacked-diffs-versus-pull-requests/

Short explanation: A git flow where the atomic unit of change is a commit. Each commit runs all the tests of ci and gets reviewed. When working with stacked diffs or commits, to add more stuff to a particular commit, you amend that commit. 

## Stacked Diffs on GitHub
    
The github model is not quite compatible with the stacked diffs model. At the core the atomic unit of change is a pull request which can have many commits. 
**git pr** is a script trying to achieve a stacked diff workflow using github pull requests and branches. 

## Installation 

Install the git commit hook
```bash
~/apomelo> ln -s ../../s/commit_msg_hook .git/hooks/commit-msg
```

Add the script directory to your path, make sure to replace HOMEDIR with the actual path to the apomelo directory.
```bash
> export PATH="~/HOMEDIR/apomelo/s:${PATH}"
```

Set github token. Make a new token here: https://github.com/settings/tokens. Use the **repo** auth scope.
```bash
> export GITHUB_TOKEN="token value"
```

Add the export statements into your `.bash_profile` so they get set each time you open a new terminal. 

<details><summary>Fish instructions</summary>

Add script directory to your path:
```fish
set fish_user_paths ~/apomelo/s $fish_user_paths
```

Set github token
```fish
set -Ux GITHUB_TOKEN "token value"
```
</details>

    
## Workflow
Create a branch to work in. 
```shell
> git branch -c salmon
```

In the stacked diff workflow you don't need to create a branch for every pull request or thing you want to do. You will generally work in one branch, but you can still create multiple branches and separate features into different branches if you want.
    
Commit your changes to the branch. Note that every commit will end up being a pull request with one commit inside. 
```shell
> git add ..... 
> git commit  
```

Write a good commit message explaining the change. The subject of the commit message will be the title of the PR, and the body of the message will be the body of the PR. 
If you have a work in progress change that you want to commit, but don't want to create a pull request yet, start the commit message with all caps **WIP**.

## Amending Commits
When you need to update a commit, either to fix tests, update code based on review comments, or just need to change something because you feel like it. You should amend the commit. 
Use the amend script to easily amend your changes anywhere in the stack. 
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

## Managing Pull Requests
Run **git pr** to sync your whole commit stack to github and create pull requests for each commit in the stack. If a commit was amended the pull request will be updated automatically. The command outputs a list of your open pull requests and their status. The command takes a few seconds to run.

```shell
> git pr -u
[XVX] 60: Feature C
[VVV] 59: Feature B
[VVV] 58: Feature A
```

## Merging Pull Requests
Your pull requests are stacked. Don't use the UI to merge pull requests, if you do it in the wrong order, you'll end up pushing one pull request into another. Just use the --merge option and you can merge all the pull requests that are mergeable. A pull request is mergeable if it:
- doesn't have conflicts with master branch
- passed build and github checks
- has at least one pull request approval 

```shell
> git pr --merge
#58 (merged)    Feature A
#59 (merged)    Feature B
#60 (unchanged) Feature C
```

If you made it all the way here, great job! 
Now add another exclamation mark after Happy Coding on the next line, commit the change, and run **git pr** to create a pull request.

## Happy Coding! 
