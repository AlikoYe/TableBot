const categories = {
  restaurant: { label: 'Restaurants', icon: '' },
  cafe: { label: 'Cafes', icon: '' },
  bar: { label: 'Bars', icon: '' },
  billiards: { label: 'Billiards', icon: '' },
  karaoke: { label: 'Karaoke', icon: '' },
  other: { label: 'Other', icon: '' },
}

export const categoryLabel = (c) => categories[c]?.label ?? 'Other'
export const categoryIcon = (c) => categories[c]?.icon ?? '📍'
export const categoryOrder = Object.keys(categories)

export const hoursText = (v) => `${v.opens_at}–${v.closes_at}`
