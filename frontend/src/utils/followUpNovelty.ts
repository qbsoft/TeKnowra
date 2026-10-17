export function excludeAskedQuestions<T extends { text: string }>(items: T[], asked: string[]): T[] {
  const normalize = (text: string) => text.toLowerCase().replace(/[\s?？!！,，.。:：;；"']/g, '')
  const keys = new Set(asked.map(normalize))
  return items.filter(item => !keys.has(normalize(item.text)))
}
