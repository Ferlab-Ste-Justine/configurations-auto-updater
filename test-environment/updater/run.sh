#!/usr/bin/env sh

(cd ../..; go build .; cp configurations-auto-updater test-environment/updater/configurations-auto-updater)

CONFS_AUTO_UPDATER_CONFIG_FILE=config.yml ./configurations-auto-updater