#!/usr/bin/python
# Copyright 2023 Northern.tech AS
#
#    Licensed under the Apache License, Version 2.0 (the "License");
#    you may not use this file except in compliance with the License.
#    You may obtain a copy of the License at
#
#        http://www.apache.org/licenses/LICENSE-2.0
#
#    Unless required by applicable law or agreed to in writing, software
#    distributed under the License is distributed on an "AS IS" BASIS,
#    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#    See the License for the specific language governing permissions and
#    limitations under the License.
import json
import os

import pytest

import cli
from common import DEFAULT_TOKEN_PATH


@pytest.fixture(scope="function")
def logged_in_single_user(single_user):
    c = cli.Cli()
    r = c.run(
        "login",
        "--server",
        "https://docker.mender.io",
        "--skip-verify",
        "--username",
        "user@tenant.com",
        "--password",
        "youcantguess",
    )

    assert r.returncode == 0, r.stderr
    yield
    os.remove(DEFAULT_TOKEN_PATH)


class TestInventoryDevicesList:
    def test_list_raw_ok(self, logged_in_single_user):
        c = cli.Cli()
        r = c.run(
            "--server",
            "https://docker.mender.io",
            "--skip-verify",
            "inventory",
            "devices",
            "list",
            "--raw",
        )
        assert r.returncode == 0, r.stderr
        data = json.loads(r.stdout)
        assert isinstance(data, list)

    def test_list_with_filter_ok(self, logged_in_single_user):
        c = cli.Cli()
        r = c.run(
            "--server",
            "https://docker.mender.io",
            "--skip-verify",
            "inventory",
            "devices",
            "list",
            "--raw",
            "--filter",
            "inventory/nonexistent_cli_test_attr=foo",
        )
        assert r.returncode == 0, r.stderr
        data = json.loads(r.stdout)
        assert isinstance(data, list)

    def test_list_pretty_ok(self, logged_in_single_user):
        c = cli.Cli()
        r = c.run(
            "--server",
            "https://docker.mender.io",
            "--skip-verify",
            "inventory",
            "devices",
            "list",
        )
        assert r.returncode == 0, r.stderr
