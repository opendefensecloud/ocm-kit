# Contributing

## Commit Convention

This project follows the [Conventional Commits](https://www.conventionalcommits.org/) specification. Both PR titles and individual commit messages are validated in CI.

### Format

```
<type>(optional scope): <description>
```

### Allowed Types

| Type       | Purpose                                              |
| ---------- | ---------------------------------------------------- |
| `feat`     | A new feature                                        |
| `fix`      | A bug fix                                            |
| `docs`     | Documentation changes                                |
| `chore`    | Maintenance tasks (deps, CI config, etc.)            |
| `refactor` | Code changes that neither fix a bug nor add a feature |
| `test`     | Adding or updating tests                             |
| `ci`       | CI/CD pipeline changes                               |
| `perf`     | Performance improvements                             |
| `revert`   | Reverting a previous commit                          |

### Examples

```
feat: add OCI artifact signing support
fix(registry): handle missing manifest digest
docs: update contributing guidelines
chore(deps): update cosign to v2.5.0
refactor(transfer): extract blob streaming logic
```

### Breaking Changes

Append `!` after the type/scope to indicate a breaking change:

```
feat!: change artifact transfer API
refactor(api)!: rename TransferPolicy to SyncPolicy
```
