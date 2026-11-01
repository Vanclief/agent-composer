// Items read as thoughts (ADR 0006, decision 11): "I believe …", "I think
// …, but have no evidence yet", "ana believes …" when a belief has by, "I
// believe …, from franco's experience in …" for a judgment, "I decided that …", "I will …", "<who> saw
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

// Who holds a belief, then verb as they say it: "I believe" when by is
// empty (the repo owner), "ana believes" otherwise, like holder in
// knowledge/sentence.go.
function holder(belief: Belief, verb: string) {
  return belief.by ? `${belief.by} ${conjugate(belief, verb)}` : `I ${verb}`
}

function conjugate(belief: Belief, verb: string) {
  if (!belief.by || verb === 'believed') {
    return verb
  }
  return verb === 'have' ? 'has' : `${verb}s`
}

// Whose experience a judgment comes from, and in what, folded to one line
// without a closing period.
function fromExperience(belief: Belief) {
  const { experience } = belief
  if (!experience) {
    return ''
  }
  const what = experience.in.split(/\s+/).filter(Boolean).join(' ').replace(/\.$/, '')
  return `, from ${experience.of}'s experience in ${what}`
}

export function beliefSentence(belief: Belief, proper: Set<string>, without: string[]) {
  const t = embed(belief.text, proper)
  switch (belief.label) {
    case 'opinion':
      return `${holder(belief, 'think')} ${t}, but ${conjugate(belief, 'have')} no evidence yet.`
    case 'judgment':
      return `${holder(belief, 'believe')} ${t}${fromExperience(belief)}.`
    case 'in_doubt':
      // In a what-if with nothing really retracted under it, it is only
      // doubted because of what was set aside.
      if ((belief.retracted ?? []).length === 0 && without.length > 0) {
        return `${belief.by || 'I'} would doubt that ${t}, without ${without.join(', ')}.`
      }
      return `${holder(belief, 'believed')} ${t}, but its evidence was retracted.`
    case 'contested':
      // Contested hides whether it rests on an opinion; the verb keeps it.
      return `${holder(belief, belief.opinions.length > 0 ? 'think' : 'believe')} ${t}, though it is contested.`
    default:
      return `${holder(belief, 'believe')} ${t}.`
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
    return `${holder(belief, 'think')} ${t}`
  }
  if (belief.label === 'in_doubt') {
    return `${holder(belief, 'believed')} ${t}`
  }
  if (belief.label === 'judgment') {
    return `${holder(belief, 'believe')} ${t}${fromExperience(belief)}`
  }
  return `${holder(belief, 'believe')} ${t}`
}
