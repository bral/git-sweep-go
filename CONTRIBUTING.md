# Contributing to git-sweep-go

Thank you for considering contributing to `git-sweep-go`!

## How to Contribute

1.  **Fork the Repository:** Start by forking the main repository on GitHub.
2.  **Clone Your Fork:** Clone your forked repository to your local machine.
    ```bash
    git clone https://github.com/YOUR_USERNAME/git-sweep-go.git
    cd git-sweep-go
    ```
3.  **Create a Branch:** Create a new branch for your feature or bug fix.
    ```bash
    git checkout -b your-feature-name
    ```
4.  **Make Changes:** Implement your changes or fixes.
5.  **Run Tests:** Ensure all unit tests pass.
    ```bash
    go test ./...
    ```
6.  **Commit Changes:** Commit your changes following the required format: `type(scope): message`.
    ```bash
    git commit -am "update(tui): improve branch deletion UI"
    git commit -am "fix(gitcmd): resolve fetch error handling"
    ```
    
    Valid types include:
    - `add`: New features or files
    - `update`: Improvements to existing features
    - `fix`: Bug fixes
    - `docs`: Documentation only changes
    - `style`: Formatting, missing semicolons, etc; no code change
    - `refactor`: Code changes that neither fix bugs nor add features
    - `test`: Adding or updating tests
    - `chore`: Changes to the build process or auxiliary tools
    - `feat`: Alternative to 'add' for new features
    - `perf`: Performance improvements
7.  **Push to Your Fork:** Push your changes to your forked repository.
    ```bash
    git push origin your-feature-name
    ```
8.  **Open a Pull Request:** Go to the original repository on GitHub and open a pull request from your branch to the main branch. Provide a clear description of your changes in the pull request.

## Reporting Issues

If you encounter a bug or have a feature request, please open an issue on the GitHub repository. Provide as much detail as possible, including steps to reproduce if reporting a bug.

Thank you for your contribution!
