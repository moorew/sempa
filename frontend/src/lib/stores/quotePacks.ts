// Curated quote & affirmation "packs" for the daily encouragement feature.
//
// IMPORTANT — provenance: every attributed quote in here was fact-checked against
// reliable sources (Wikiquote sourced sections, primary works, documented
// speeches/letters/interviews). The notorious misattributions — the fake
// Churchill / Lincoln / Einstein / Gandhi / Aristotle lines — were deliberately
// dropped, and a few were re-credited to their real author (e.g. "Price is what
// you pay…" → Benjamin Graham, not Buffett; the "excellence is a habit" line →
// Will Durant, not Aristotle). Please keep that bar: only add a quote here if you
// can point to a real source for it. Made-up or unverifiable quotes do not belong.
//
// Affirmations are original, deliberately factual and grounding rather than
// aspirational fluff — they carry no author (shown without an attribution line).

export interface Quote {
  text: string;
  /** Empty string for affirmations (rendered with no attribution line). */
  author: string;
}

export interface QuotePack {
  id: string;
  name: string;
  /** One short line shown under the pack name in Settings. */
  description: string;
  quotes: Quote[];
}

export const QUOTE_PACKS: QuotePack[] = [
  {
    id: 'business',
    name: 'Business & leadership',
    description: 'Founders, executives and management thinkers.',
    quotes: [
      { text: 'It takes 20 years to build a reputation and five minutes to ruin it.', author: 'Warren Buffett' },
      { text: 'Price is what you pay. Value is what you get.', author: 'Benjamin Graham' },
      { text: 'Your work is going to fill a large part of your life, and the only way to be truly satisfied is to do what you believe is great work.', author: 'Steve Jobs' },
      { text: 'There is only one boss. The customer.', author: 'Sam Walton' },
      { text: "If you are not embarrassed by the first version of your product, you've launched too late.", author: 'Reid Hoffman' },
      { text: 'I never dreamed about success. I worked for it.', author: 'Estée Lauder' },
      { text: "In times of adversity and change, we really discover who we are and what we're made of.", author: 'Howard Schultz' },
      { text: 'Don’t limit yourself. Many people limit themselves to what they think they can do.', author: 'Mary Kay Ash' },
      { text: 'Leadership is the capacity to translate vision into reality.', author: 'Warren Bennis' },
      { text: 'Chase the vision, not the money; the money will end up following you.', author: 'Tony Hsieh' },
    ],
  },
  {
    id: 'history',
    name: 'History & statecraft',
    description: 'Leaders and reformers, in their own documented words.',
    quotes: [
      { text: 'Never give in. Never give in. Never, never, never, never.', author: 'Winston Churchill' },
      { text: 'Now this is not the end. It is not even the beginning of the end. But it is, perhaps, the end of the beginning.', author: 'Winston Churchill' },
      { text: 'I learned that courage was not the absence of fear, but the triumph over it.', author: 'Nelson Mandela' },
      { text: 'The weak can never forgive. Forgiveness is the attribute of the strong.', author: 'Mahatma Gandhi' },
      { text: 'You must do the things you think you cannot do.', author: 'Eleanor Roosevelt' },
      { text: 'It is not the critic who counts. The credit belongs to the one who is actually in the arena.', author: 'Theodore Roosevelt' },
      { text: 'If there is no struggle, there is no progress.', author: 'Frederick Douglass' },
      { text: 'Ask not what your country can do for you — ask what you can do for your country.', author: 'John F. Kennedy' },
      { text: 'Failure is impossible.', author: 'Susan B. Anthony' },
      { text: "I'd rather go down in history as one lone Negro who dared to tell the government that it had done a dastardly thing than to save my skin by taking back what I said.", author: 'Ida B. Wells' },
    ],
  },
  {
    id: 'science',
    name: 'Science & discovery',
    description: 'Scientists and explorers on curiosity and truth.',
    quotes: [
      { text: 'We are made of star-stuff.', author: 'Carl Sagan' },
      { text: 'Extraordinary claims require extraordinary evidence.', author: 'Carl Sagan' },
      { text: 'The important thing is not to stop questioning. Curiosity has its own reason for existing.', author: 'Albert Einstein' },
      { text: 'The first principle is that you must not fool yourself — and you are the easiest person to fool.', author: 'Richard Feynman' },
      { text: 'If I have seen further it is by standing on the shoulders of giants.', author: 'Isaac Newton' },
      { text: 'A man who dares to waste one hour of time has not discovered the value of life.', author: 'Charles Darwin' },
      { text: 'What you do makes a difference, and you have to decide what kind of difference you want to make.', author: 'Jane Goodall' },
      { text: 'Look up at the stars and not down at your feet.', author: 'Stephen Hawking' },
      { text: 'Science and everyday life cannot and should not be separated.', author: 'Rosalind Franklin' },
      { text: "The good thing about science is that it's true whether or not you believe in it.", author: 'Neil deGrasse Tyson' },
      { text: 'In the field of observation, chance favours only the prepared mind.', author: 'Louis Pasteur' },
      { text: 'Equipped with his five senses, man explores the universe around him and calls the adventure Science.', author: 'Edwin Hubble' },
    ],
  },
  {
    id: 'literature',
    name: 'Arts & literature',
    description: 'Writers and artists on creativity and resilience.',
    quotes: [
      { text: "If there's a book that you want to read, but it hasn't been written yet, then you must write it.", author: 'Toni Morrison' },
      { text: 'Courage is resistance to fear, mastery of fear — not absence of fear.', author: 'Mark Twain' },
      { text: 'We are all in the gutter, but some of us are looking at the stars.', author: 'Oscar Wilde' },
      { text: 'It is impossible to live without failing at something, unless you live so cautiously that you might as well not have lived at all.', author: 'J.K. Rowling' },
      { text: 'The world breaks everyone, and afterward many are strong at the broken places.', author: 'Ernest Hemingway' },
      { text: 'Great things are not done by impulse, but by a series of small things brought together.', author: 'Vincent van Gogh' },
      { text: 'Inspiration exists, but it has to find you working.', author: 'Pablo Picasso' },
      { text: 'There is no gate, no lock, no bolt that you can set upon the freedom of my mind.', author: 'Virginia Woolf' },
      { text: 'You must stay drunk on writing so reality cannot destroy you.', author: 'Ray Bradbury' },
      { text: 'A word after a word after a word is power.', author: 'Margaret Atwood' },
      { text: 'There is no greater agony than bearing an untold story inside you.', author: 'Zora Neale Hurston' },
    ],
  },
  {
    id: 'culture',
    name: 'Stage & screen',
    description: 'Actors, musicians and cultural voices.',
    quotes: [
      { text: 'Look for the helpers. You will always find people who are helping.', author: 'Fred Rogers' },
      { text: 'The biggest adventure you can take is to live the life of your dreams.', author: 'Oprah Winfrey' },
      { text: 'Turn your wounds into wisdom.', author: 'Oprah Winfrey' },
      { text: 'Find out who you are and do it on purpose.', author: 'Dolly Parton' },
      { text: 'Ease is a greater threat to progress than hardship.', author: 'Denzel Washington' },
      { text: 'The only thing that separates women of color from anyone else is opportunity.', author: 'Viola Davis' },
      { text: "Don't you ever let a soul in the world tell you that you can't be exactly who you are.", author: 'Lady Gaga' },
      { text: "I have standards I don't plan on lowering for anybody, including myself.", author: 'Zendaya' },
      { text: 'We need to reshape our own perception of how we view ourselves.', author: 'Beyoncé' },
    ],
  },
  {
    id: 'sport',
    name: 'Sport',
    description: 'Athletes and coaches on practice and grit.',
    quotes: [
      { text: "I've missed more than 9,000 shots in my career. I've lost almost 300 games. I've failed over and over and over again in my life. And that is why I succeed.", author: 'Michael Jordan' },
      { text: "You miss 100% of the shots you don't take.", author: 'Wayne Gretzky' },
      { text: 'Float like a butterfly, sting like a bee.', author: 'Muhammad Ali' },
      { text: "I hated every minute of training, but I said, 'Don't quit. Suffer now and live the rest of your life as a champion.'", author: 'Muhammad Ali' },
      { text: "Winning isn't everything, but wanting to win is.", author: 'Vince Lombardi' },
      { text: 'Success is no accident. It is hard work, perseverance, learning, studying, sacrifice and most of all, love of what you are doing.', author: 'Pelé' },
      { text: 'A champion is defined not by their wins but by how they can recover when they fall.', author: 'Serena Williams' },
      { text: "It's hard to beat a person who never gives up.", author: 'Babe Ruth' },
      { text: "It ain't over till it's over.", author: 'Yogi Berra' },
      { text: 'When you come to a fork in the road, take it.', author: 'Yogi Berra' },
      { text: 'Be water, my friend.', author: 'Bruce Lee' },
      { text: 'I fear not the man who has practiced 10,000 kicks once, but I fear the man who has practiced one kick 10,000 times.', author: 'Bruce Lee' },
      { text: "Hard work beats talent when talent doesn't work hard.", author: 'Tim Notke' },
    ],
  },
  {
    id: 'philosophy',
    name: 'Philosophy',
    description: 'Philosophers, ancient and modern, on living well.',
    quotes: [
      { text: 'Waste no more time arguing about what a good man should be. Be one.', author: 'Marcus Aurelius' },
      { text: 'We suffer more often in imagination than in reality.', author: 'Seneca' },
      { text: 'It is not that we have a short time to live, but that we waste a lot of it.', author: 'Seneca' },
      { text: 'We are what we repeatedly do. Excellence, then, is not an act, but a habit.', author: 'Will Durant' },
      { text: 'The unexamined life is not worth living.', author: 'Socrates' },
      { text: 'The journey of a thousand miles begins with a single step.', author: 'Lao Tzu' },
      { text: "Real knowledge is to know the extent of one's ignorance.", author: 'Confucius' },
      { text: 'He who has a why to live for can bear almost any how.', author: 'Friedrich Nietzsche' },
      { text: 'When we are no longer able to change a situation, we are challenged to change ourselves.', author: 'Viktor Frankl' },
      { text: "Everything can be taken from a man but one thing: the last of the human freedoms — to choose one's attitude in any given set of circumstances.", author: 'Viktor Frankl' },
      { text: 'Life can only be understood backwards; but it must be lived forwards.', author: 'Søren Kierkegaard' },
      { text: 'Act as if what you do makes a difference. It does.', author: 'William James' },
    ],
  },
  {
    id: 'affirmations',
    name: 'Affirmations',
    description: 'Calm, factual reminders — no slogans, no author.',
    quotes: [
      { text: 'Starting and finishing are separate jobs. Right now, you only need to start.', author: '' },
      { text: 'A task feels smaller the moment it has a first step.', author: '' },
      { text: 'You have handled every hard day so far. That is a real track record.', author: '' },
      { text: "Progress still counts on the days it isn't visible.", author: '' },
      { text: 'You can only do one thing at a time, and one thing is enough.', author: '' },
      { text: 'Rest is part of the work, not time stolen from it.', author: '' },
      { text: 'Done is more useful than perfect.', author: '' },
      { text: "You don't have to feel ready in order to begin.", author: '' },
      { text: 'Most of what we worry about never happens.', author: '' },
      { text: 'Focus comes back when you remove one distraction at a time.', author: '' },
      { text: 'Small actions, repeated, outlast a burst of motivation.', author: '' },
      { text: "You're allowed to change your mind when you have better information.", author: '' },
      { text: 'Saying no to one thing is saying yes to another.', author: '' },
      { text: 'What you give attention to tends to grow.', author: '' },
      { text: 'A clear next step is worth more than a perfect plan.', author: '' },
      { text: 'Mistakes are information, not a verdict.', author: '' },
    ],
  },
];

// Packs enabled out of the box. Tasteful, broad, all real people; the rest
// (philosophy, stage & screen, sport, affirmations) are opt-in via Settings.
export const DEFAULT_ENABLED_PACKS = ['business', 'history', 'science', 'literature'];

export function packById(id: string): QuotePack | undefined {
  return QUOTE_PACKS.find((p) => p.id === id);
}
