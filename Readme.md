A simple Go command line tool that cleans up obsolete amis and snapshots, and replace ecs cluster instances with newest AMI.


### Installation

```
make install
```

### Usage

#### Subccomands

- `rmi` delete images before specified generations.
- `rms` remove snapshots that is not reffered by any AMIs or volumes.
- `rpl` replace ecs cluster instances with newest AMI.

#### Options

- `rmi`
  - `image,i` prefix of AMI.
  - `owner,o` account ID of ami owner.
  - `dry-run,d` dry run flag.
  - `gen,g` max generations to retain.
  - `verbose,v` enable debug output.

- `rms`
  - `owner,o` account ID of ami owner.
  - `dry-run,d` dry run flag.
  - `verbose,v` enable debug output.

- `rpl`
  - `asgname` asg name.
  - `clustername` ecs cluster name.
  - `image,i` prefix of AMI.
  - `owner,o` account ID of ami owner.
  - `dry-run,d` dry run flag.
  - `verbose,v` enable debug output.


#### Example

Delete amis older than specified generations.
```
ami-replacer rmi --image <image name> --owner <owner> --gen=<generation> --dry-run
```


Delete unused snapshots.
```
ami-replacer rms --owner <owner> --dry-run
```


Replace ECS cluster Instances with newest AMI.
```
ami-replacer replace --image <image name>  --owner <owner> --asgname <asg name> --clustername <cluster name> -v --dry-run
```

### Change Logs

#### 0.1
 - First beta release

### Contribution
Feel free to help out by sending pull requests or by creating new issues.

### Author
Tetsuhito Yasuno(tyasuno)

### License
MIT
