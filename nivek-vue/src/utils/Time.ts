
export function formatDate(isoDate: string): string {
    const date = new Date(isoDate)

    if (isNaN(date.getTime())) {
        throw new Error('Invalid date format')
    }

    const month = (date.getMonth() + 1).toString().padStart(2, '0')
    const day = date.getDate().toString().padStart(2, '0')
    const year = date.getFullYear().toString()

    let hours = date.getHours()
    const minutes = date.getMinutes().toString().padStart(2, '0')
    const ampm = hours >= 12 ? 'PM' : 'AM'

    hours = hours % 12 || 12
    return `${month}-${day}-${year} ${hours}:${minutes}${ampm}`
}