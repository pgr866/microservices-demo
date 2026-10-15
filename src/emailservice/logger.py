#!/usr/bin/python
#
# Copyright 2018 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import logging
import os
import sys
from datetime import UTC, datetime

from pythonjsonlogger import json as jsonlogger

# Case-insensitive; empty or unset means info.
LOG_LEVELS = {
    "debug": logging.DEBUG,
    "info": logging.INFO,
    "warn": logging.WARNING,
    "error": logging.ERROR,
}


class CustomJsonFormatter(jsonlogger.JsonFormatter):
    def add_fields(self, log_record, record, message_dict):
        super().add_fields(log_record, record, message_dict)
        if not log_record.get("timestamp"):
            log_record["timestamp"] = datetime.fromtimestamp(
                record.created, UTC
            ).isoformat()
        if True if log_record.get("severity") else False:
            log_record["severity"] = log_record["severity"].upper()
        else:
            log_record["severity"] = record.levelname


def getJSONLogger(name: str) -> logging.Logger:
    logger = logging.getLogger(name)
    # getLogger returns the same logger: a second handler would duplicate lines.
    if not logger.handlers:
        handler = logging.StreamHandler(sys.stdout)
        formatter = CustomJsonFormatter(
            "%(timestamp)s %(severity)s %(name)s %(message)s"
        )
        handler.setFormatter(formatter)
        logger.addHandler(handler)
    # Info for an unknown value, so exit_on_invalid_log_level can still report it.
    logger.setLevel(log_level() or logging.INFO)
    logger.propagate = False
    return logger


def log_level() -> int | None:
    return LOG_LEVELS.get((os.environ.get("LOG_LEVEL") or "info").lower())


def exit_on_invalid_log_level(logger: logging.Logger) -> None:
    """Exits with one JSON log line, and no traceback, on an unknown LOG_LEVEL."""
    if log_level() is None:
        logger.error(
            "invalid LOG_LEVEL %r: expected one of %s",
            os.environ["LOG_LEVEL"],
            ", ".join(LOG_LEVELS),
        )
        sys.exit(1)
