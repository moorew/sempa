import { describe, it, expect } from 'vitest';
import { summarise, LEARN_MIN_SAMPLES, LEARN_MAX_SPREAD } from './stores/timeProfile.svelte';
import type { DurationSample } from './types';

const sample = (title: string, minutes: number, tags: string[] = []): DurationSample => ({
  title, minutes, tags,
});

// Repeat a set of durations for one kind of work.
const emails = (mins: number[]) => mins.map((m, i) => sample(`Reply to email ${i}`, m));

describe('time-learning gates', () => {
  it('keeps asking until there are enough samples', () => {
    const stats = summarise(emails(Array(LEARN_MIN_SAMPLES - 1).fill(12)));
    expect(stats.get('email')?.learned).toBe(false);
  });

  it('learns a consistent bucket once the sample bar is met', () => {
    const stats = summarise(emails([12, 11, 13, 12, 14]));
    const email = stats.get('email')!;
    expect(email.samples).toBe(5);
    expect(email.learned).toBe(true);
    expect(email.median).toBe(12);
  });

  // The gate that stops the feature doing harm: a bucket with plenty of history
  // but wildly varying durations has a median, not a prediction. Auto-filling it
  // would put fiction on the task and feed that fiction back into the profile.
  it('refuses to learn a bucket whose durations are all over the place', () => {
    const stats = summarise([
      sample('Build the importer', 15),
      sample('Build the auth flow', 240),
      sample('Debug the sync race', 30),
      sample('Design the schema', 180),
      sample('Refactor the poller', 20),
      sample('Implement the parser', 300),
    ]);
    const deep = stats.get('deepwork')!;
    expect(deep.samples).toBeGreaterThanOrEqual(LEARN_MIN_SAMPLES);
    expect(deep.spread).toBeGreaterThan(LEARN_MAX_SPREAD);
    expect(deep.learned).toBe(false);
  });

  it('treats a forgotten bucket as unlearned so it can be re-taught', () => {
    const data = emails([12, 11, 13, 12, 14]);
    expect(summarise(data).get('email')?.learned).toBe(true);
    expect(summarise(data, new Set(['email'])).get('email')?.learned).toBe(false);
  });

  it('ignores rows with no logged time', () => {
    const stats = summarise([...emails([12, 12, 12, 12, 12]), sample('Reply to nobody', 0)]);
    expect(stats.get('email')?.samples).toBe(5);
  });

  it('keeps buckets independent', () => {
    const stats = summarise([
      ...emails([10, 10, 10, 10, 10]),
      sample('Buy groceries', 45),
    ]);
    expect(stats.get('email')?.learned).toBe(true);
    expect(stats.get('errand')?.learned).toBe(false);
  });
});
