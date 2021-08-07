![logo](git_spr_logo.png)
## Stacked Pull Requests on GitHub

I am a total practitioner of writing code in small consumable chunks, getting them reviewed and merged quickly. This approach leads to very fast paced code iterations, merge a lot and merge often. There is no question in my mind that writing software in this way is more productive than long lived branches with pull requests taking days or weeks to merge.

GitHub is an amazing platform, it has revolutionized software development and created a vibrant open source community. And yet, I found managing branches and stacks of pull requests on GitHub to be somewhere between annoying and unbearable. As the stacks of pull requests get bigger, you end up spending more and more time switching things around with a high risk of messing things up. If you want to merge something in the middle of the stack, you are in for a fun ride. Many teams avoid stacked pull requests altogether because of the added complexities and have forgotten the joy of being able to work in composed units.

I created SPR to solve this, a simple tool which does all the branch and pull request management for you, so you can just focus on your code and not have to spend time rebranching, updating and managing stacks of pull requests.

With SPR each git commit is an atomic unit of work that needs to pass all the checks and go through a pull request approval process. This ensures that the repository mainline never breaks in a sense that all tests run and pass on every commit This unlocks simplified deployment debugging with tools such as git bisect to pinpoint a failure to a particular commit. It also enforces a clean commit lineage in the repository mainline with no merge commits.

SPR is a client side script integrated into git, there is no special configuration on GitHub or any special server side software that needs to run. It can be used in any GitHub repository and doesn't interfere with other workflows as it uses the same pull request model.

![asciicasst](git_spr_cast.gif)
--------------------------------------

To create a pull request. Just commit your changes and call spr update. The branch is pushed to GitHub, and a pull request is created. 
```shell
> git commit -m "Feature A"
> git spr update
[--✔✔] github.com/ejoffe/spr-demo/pull/1 : Feature A
```
You can add and amend commits and a call to spr update will synchronize your pull requests to your current commit stack. You can even reorder commits and your pull request's branches will be correctly updated. 
```shell
> git commit -m "feature B"
> ...
> git commit -m "Feature C"
> git spr update
[✗✗✔✔] github.com/ejoffe/spr-demo/pull/2 : Feature C
[✗✗✔✔] github.com/ejoffe/spr-demo/pull/2 : Feature B
[✔✔✔✔] github.com/ejoffe/spr-demo/pull/1 : Feature A
```
Once a pull request has four check marks it means it has passed checks, got an approval, and has no conflicts. Your pull request is ready to merge. Use spr merge to merge all ready pull requests. Request approval and checks requirement can be disabled in the 
```shell
> git spr merge
MERGED github.com/ejoffe/spr-demo/pull/1 : Feature A

[✗✗✔✔] github.com/ejoffe/spr-demo/pull/2 : Feature C
[✗✗✔✔] github.com/ejoffe/spr-demo/pull/2 : Feature B
```

Find out more here : [https://github.com/ejoffe/spr](https://github.com/ejoffe/spr)

**Happy Coding!**
