// Items read as thoughts (ADR 0006, decision 11): "I believe …", "I think
// …, but have no evidence yet", "I decided that …", "I will …", "<who> saw
// that … on …". Built from the texts and the server's label with fixed
// templates, like knowledge/sentence.go; the raw text goes in a tooltip.
import type { Action, Belief, Evidence, Knowledge } from '@/api/knowledge'

// Words written capitalized in the middle of a sentence somewhere in the
// loaded texts, such as Go or Task: names, which keep their capital when a
// text is embedded.
export function properWords(data: Knowledge): Set<string> {
  const texts = [
    ...data.evidence.map((e) => e.text),
    ...data.beliefs.map((b) => b.text),
    ...data.decisions.map((d) => d.text),
    ...data.rules.map((r) => r.text),
    ...(data.actions ?? []).map((a) => a.text),
  ]
  const proper = new Set<string>()
  for (const text of texts) {
    const words = text.split(/\s+/).filter(Boolean)
    words.forEach((word, i) => {
      if (i === 0 || /[.!?]$/.test(words[i - 1])) {
        return
      }
      if (/^\p{Lu}/u.test(word)) {
        proper.add(properKey(word))
      }
    })
  }
  return proper
}

// A word without punctuation around it or a possessive 's.
function properKey(word: string) {
  return word.replace(/^[^\p{L}\p{N}]+|[^\p{L}\p{N}]+$/gu, '').replace(/['’]s$/, '')
}

// A text as a clause: one line, no closing period, and its first letter
// lower case unless the first word is a name (capitalized mid-sentence
// somewhere, an inner capital as in SQLite, or a single letter but A).
export function embed(text: string, proper: Set<string>) {
  const clause = text.split(/\s+/).filter(Boolean).join(' ').replace(/\.$/, '')
  const first = clause.split(' ')[0] ?? ''
  const key = properKey(first)
  const letter = [...key].length === 1 && key !== 'A'
  if (!clause || proper.has(key) || /\p{Lu}/u.test(first.slice(1)) || letter) {
    return clause
  }
  return clause.charAt(0).toLowerCase() + clause.slice(1)
}

export function beliefSentence(belief: Belief, proper: Set<string>, without: string[]) {
  const t = embed(belief.text, proper)
  switch (belief.label) {
    case 'opinion':
      return `I think ${t}, but have no evidence yet.`
    case 'in_doubt':
      // In a what-if with nothing really retracted under it, it is only
      // doubted because of what was set aside.
      if ((belief.retracted ?? []).length === 0 && without.length > 0) {
        return `I would doubt that ${t}, without ${without.join(', ')}.`
      }
      return `I believed ${t}, but its evidence was retracted.`
    case 'contested':
      // Contested hides whether it rests on an opinion; the verb keeps it.
      return `${belief.opinions.length > 0 ? 'I think' : 'I believe'} ${t}, though it is contested.`
    default:
      return `I believe ${t}.`
  }
}

export function decisionSentence(text: string, proper: Set<string>) {
  return `I decided that ${embed(text, proper)}.`
}

export function actionSentence(action: Action, proper: Set<string>) {
  return `${action.done ? 'I did' : 'I will'} ${embed(action.text, proper)}.`
}

export function evidenceSentence(evidence: Evidence, proper: Set<string>) {
  const by = evidence.translated_by ? `, written up by ${evidence.translated_by}` : ''
  return `${evidence.who} saw that ${embed(evidence.text, proper)} on ${evidence.when}${by}.`
}

// One side of a conflict as a clause, without the closing period.
export function sideClause(id: string, data: Knowledge, proper: Set<string>) {
  const evidence = data.evidence.find((e) => e.id === id)
  if (evidence) {
    return evidenceSentence(evidence, proper).replace(/\.$/, '')
  }
  const belief = data.beliefs.find((b) => b.id === id)
  if (!belief) {
    return id
  }
  const t = embed(belief.text, proper)
  if (belief.label === 'opinion' || (belief.label === 'contested' && belief.opinions.length > 0)) {
    return `I think ${t}`
  }
  if (belief.label === 'in_doubt') {
    return `I believed ${t}`
  }
  return `I believe ${t}`
}
