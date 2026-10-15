const createLogger = require('./logger');

describe('createLogger', () => {
  const original = process.env.LOG_LEVEL;

  afterEach(() => {
    if (original === undefined) {
      delete process.env.LOG_LEVEL;
    } else {
      process.env.LOG_LEVEL = original;
    }
  });

  function withLevel(value) {
    if (value === undefined) {
      delete process.env.LOG_LEVEL;
    } else {
      process.env.LOG_LEVEL = value;
    }
    return createLogger('test');
  }

  it.each([
    [undefined, 'info'],
    ['', 'info'],
    ['DEBUG', 'debug'],
    ['warn', 'warn'],
    ['error', 'error'],
  ])('LOG_LEVEL=%p sets the level to %s', (value, expected) => {
    expect(withLevel(value).level).toBe(expected);
  });

  it.each(['verbose', 'trace', 'fatal'])(
    'LOG_LEVEL=%p logs one JSON line and exits with 1',
    (value) => {
      process.env.LOG_LEVEL = value;
      const exited = new Error('exited');
      const exit = jest.spyOn(process, 'exit').mockImplementation(() => {
        throw exited;
      });
      const lines = [];

      expect(() =>
        createLogger('test', { write: (line) => lines.push(line) }),
      ).toThrow(exited);

      expect(exit).toHaveBeenCalledWith(1);
      expect(lines).toHaveLength(1);
      expect(JSON.parse(lines[0])).toEqual({
        timestamp: expect.stringMatching(/Z$/),
        severity: 'error',
        message: expect.stringContaining(`"${value}"`),
        name: 'test',
      });
      exit.mockRestore();
    },
  );

  it('writes JSON lines with timestamp, severity, message and the logger name', () => {
    const lines = [];
    const logger = createLogger('test', {
      write: (line) => lines.push(JSON.parse(line)),
    });

    logger.warn('something happened');

    expect(lines).toEqual([
      expect.objectContaining({
        severity: 'warn',
        message: 'something happened',
        name: 'test',
      }),
    ]);
    expect(lines[0]).not.toHaveProperty('time');
    expect(lines[0].timestamp).toMatch(
      /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z$/,
    );
  });
});
