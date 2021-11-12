# go-tree

## Guidelines

- AddChild panics if node is closed or child name is taken.
- Processes are not closed when their node closes but they do close their node on exit.
- Children are closed asynchronously in a go routine different from the one calling their parent close. 
- Children autoremove from parent but do not close parent.
- Children that need to auto close their parent must explicitly add a closer pointing to their parent.
- Use IfRecoverX to ensure close of resources when add child or add process panics
