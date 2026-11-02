import { describe, expect, it } from 'vitest';
import { csvCell, tasksToCSV } from './export';
import { safeHref } from './utils';
import type { Task } from './types';

describe('csvCell', () => {
  it('opens formula-trigger text as text', () => {
    for (const v of ['=1+1', '+1+1', '-2+3', '@SUM(1,1)', '\t=1', '\r=1']) {
      const cell = csvCell(v);
      const unquoted = cell.startsWith('"') ? cell.slice(1, -1).replace(/""/g, '"') : cell;
      expect(unquoted.startsWith("'")).toBe(true);
    }
  });
  it('leaves ordinary text and numbers alone', () => {
    expect(csvCell('Buy milk')).toBe('Buy milk');
    expect(csvCell(-5)).toBe('-5');
    expect(csvCell('a, b')).toBe('"a, b"');
  });
  it('neutralises a formula title in a full export', () => {
    const csv = tasksToCSV([{ title: '=HYPERLINK("http://x.test","x")', status: 'planned', tags: [] } as unknown as Task]);
    expect(csv.split('\r\n')[1].startsWith(`"'=HYPERLINK`)).toBe(true);
  });
});

describe('safeHref', () => {
  it('keeps web links', () => {
    expect(safeHref('https://example.test/a')).toBe('https://example.test/a');
    expect(safeHref('http://example.test')).toBe('http://example.test/');
  });
  it('drops script and non-web schemes', () => {
    for (const v of ['javascript:alert(1)', ' JavaScript:alert(1)', 'data:text/html,x', 'vbscript:x', 'file:///etc/passwd', '/relative', '', null, undefined]) {
      expect(safeHref(v as string)).toBeUndefined();
    }
  });
});
