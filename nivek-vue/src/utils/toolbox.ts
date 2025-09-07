
export function getGreeting(date: Date = new Date()): string {
    const hour = date.getHours()
    if (hour >= 12 && hour < 18) {
        return "Good Afternoon"
    } else if (hour >= 18 || hour < 5) {
        return "Good Evening"
    } else {
        return "Good Morning"
    }
}
