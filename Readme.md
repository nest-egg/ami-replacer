A simple Go command line tool that cleans up obsolete amis and snapshots, and replace ecs cluster instances with newest AMI.


### Installation

```
make install
```

### Usage

#### Subccomands

- `rmi` delete images before specified generations.
- `rms` remove snapshots that is not reffered by any AMIs.

#### Options

- `image` prefix of AMI.
- `region` AWS region.
- `owner` account ID of ami owner.
- `profile` aws profile.
- `asgname` asg name.
- `clustername` ecs cluster name.
- `dry-run` dry run flag.

#### Example

```
ami-replacer rmi --image <image name> --owner <owner> --asgname <asg name> --dry-run
```


Delete unused snapshots
```
ami-replacer rms --owner <owner> --dry-run
```


Replace ECS cluster Instances with newest AMI
```
ami-replacer replace --image <image name>  --owner <owner> --asgname <asg name> --clustername <cluster name> -v --dry-run
```