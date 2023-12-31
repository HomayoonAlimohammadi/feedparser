from datetime import datetime

def display_time(t: datetime):
    month_names = {
        1: "January", 2: "February", 3: "March", 4: "April", 5: "May", 6: "June",
        7: "July", 8: "August", 9: "September", 10: "October", 11: "November", 12: "December"
    }

    month = t.month
    day = t.day
    year = t.year

    if 11 <= day <= 13:
        suffix = "th"
    else:
        suffix = {1: "st", 2: "nd", 3: "rd"}.get(day % 10, "th")

    hour, minute, second = t.hour, t.minute, t.second
    if hour == 0:
        hour = "00"
    if minute == 0:
        minute = "00"
    if second == 0:
        second = 0

    return f"{month_names[month]} {day}{suffix} {year} {hour}:{minute}:{second}"
