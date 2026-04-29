# Contributing to ARO HCP

Welcome to the ARO HCP project! We appreciate your interest in contributing. This guide will help you get started with the contribution process.


## Table of Contents
- [Getting Started](#getting-started)
- [Contributing Guidelines](#contributing-guidelines)
- [PR and Issue Lifecycle](#pr-and-issue-lifecycle)
- [Code of Conduct](#code-of-conduct)
- [License](#license)


## Getting Started

To contribute to ARO HCP, follow these steps:

1. Clone the ARO-HCP repository to your local machine.
2. Create a new branch for your changes.
3. Make your changes and commit them.
4. Push your changes to ARO-HCP repository.
5. Submit a pull request to the main repository.

If you create a pull request for a branch located in your GitHub fork of
ARO-HCP repository, GitHub `is_running_on_fork` check will raise an error. You
need to create your pull from ARO-HCP repository directly. See
[ARO-8846](https://issues.redhat.com/browse/ARO-8846) for details.

## Contributing Guidelines
Please follow these guidelines when contributing to ARO HCP:

- Please consider, starting with a draft PR, unless you are ready for review. If you want a early feedback,
  do not hesitate to ping the code owners.
- Write meaningful commit messages and PR description. The PR will be squashed before merging, unless
  the splitting into multiple commits is explicitly needed in order to separate changes and allow
  later `git bisect`.
- The repository is structured according to the focus areas, e.g. `api` containing all exposed API specs.
  When you contribute, please follow this structure and add your contribution to the appropriate folder.
  When in doubt, open PR early and ask for feedback.
- When applicable, please always cover new functionality with the appropriate tests.
- When adding functionality, that is not yet implemented, please write appropriate documentation.
  When in doubt, ask yourself what it took you to understand the functionality, and what would you need
  to know to use it.
- When adding new features, please consider to record a short video showing how it works and explaining
  the use case. This will help others to understand better even before digging into the code. When done,
  upload the recording to the [Drive](https://drive.google.com/drive/folders/1RB1L2-nGMXwsOAOYC-VGGbB0yD3Ae-rD?usp=drive_link) and share the link in the PR.
- When you are working on the issue that has Jira card, please always document all tradeoffs and decisions
  in the Jira card. Please, also include all design documents and slack discussion in the Jira. This will
  help others to understand the context and decisions made.

Please note, that you might be asked to comply with these guidelines before your PR is accepted.


## PR and Issue Lifecycle

This repository uses automated Prow lifecycle management to keep PRs and issues from going stale. Inactive PRs and issues progress through three stages before being automatically closed.

### Lifecycle Stages

| Stage | Label | Trigger | What happens |
|-------|-------|---------|--------------|
| **Stale** | `lifecycle/stale` | 90 days of inactivity | A comment is added warning that the PR/issue will be closed if no activity occurs |
| **Rotten** | `lifecycle/rotten` | Stale + continued inactivity | A second warning is added |
| **Closed** | — | Rotten + continued inactivity | The PR/issue is automatically closed |

Any activity (comments, pushes, label changes) resets the inactivity timer and removes the lifecycle label.

### Prow Commands

Use these commands in a PR or issue comment to manage the lifecycle:

| Command | Effect |
|---------|--------|
| `/remove-lifecycle stale` | Remove the stale label and reset the timer |
| `/remove-lifecycle rotten` | Remove the rotten label and reset the timer |
| `/lifecycle frozen` | Exempt the PR/issue from automatic closure entirely |
| `/remove-lifecycle frozen` | Remove the frozen exemption |

### Keeping Long-Running PRs Open

If you have a PR that is intentionally paused (e.g. waiting on a dependency, blocked by another team, or a long-term draft), add the `/lifecycle frozen` command to prevent it from being automatically closed.


## Code of Conduct
This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/). For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq) or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.


## License
ARO HCP is licensed under the Apache License, Version 2.0. Please see the [LICENSE](LICENSE) file for more details.
