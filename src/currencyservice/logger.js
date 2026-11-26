/*
 * Copyright 2023 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

const pino = require('pino');

// The levels shared by all the services; pino also has trace and fatal.
const LEVELS = ['debug', 'info', 'warn', 'error'];

// An unknown LOG_LEVEL logs one JSON line and exits, so the service does not start.
function createLogger(name, stream = pino.destination(1)) {
  const level = (process.env.LOG_LEVEL || 'info').toLowerCase();
  if (!LEVELS.includes(level)) {
    stream.write(
      `${JSON.stringify({
        timestamp: new Date().toISOString(),
        severity: 'error',
        message: `invalid LOG_LEVEL "${process.env.LOG_LEVEL}": must be one of ${LEVELS.join(', ')}`,
        name,
      })}\n`,
    );
    process.exit(1);
  }
  return pino(
    {
      name,
      level,
      messageKey: 'message',
      timestamp: () => `,"timestamp":"${new Date().toISOString()}"`,
      formatters: {
        level(label) {
          return { severity: label };
        },
      },
    },
    stream,
  );
}

module.exports = createLogger;
