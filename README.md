# Workspace Cleaner

This package is responsible of the process of creation/deletion/update of workspaces of Codefresh volumes
It exposes 3 capabilities
* `init` - Creates a `workspace.json`
    * requires environment variables
        * `COMMAND` ("init")
        * `WORKSPACE` - where to store `workspace.json`
* `update` - create or update an workspace in `workspace.json`
    * requires environment variables
        * `COMMAND` ("update")
        * `WORKSPACE` - where to find `workspace.json`
        * `KEY` - name of the directory to add/update , should be under the `WORKSPACE`
* `clean` - clean directories 
    * requires environment variables
        * `COMMAND` ("clean")
        * `CLEAN_STRATEGY` ( options: `perecentage` or `unused` or `key`, seperate with ":" to applay multiple)
            * when `CLEAN_STRATEGY=perecentage`, additional environment required
                * `PERCENTAGE_TO_KEEP_AVAILABLE` - number of percentage to make sure to available after the clean
            * when `CLEAN_STRATEGY=unused`, additional environment required
                * `UNUSED_N_DAYS` - number days a worksapce from `workspace.json` wasnt used to be deleted
            * when `CLEAN_STRATEGY=key`, additional environment required
                * `KEY` - keys from `workspace.json` to remove, to clean multiple, seperate the keys with `:`


## workspace.json
Definitions is [here](./main.go#L18)