function formatInTimezone(ms: number, timeZone: string): { date: string; time: string } {
  const s = new Date(ms).toLocaleString("sv-SE", { timeZone });
  const [date, time] = s.split(" ");
  return { date, time: time.slice(0, 5) };
}

/** Convert a calendar-local date+time to UTC ISO (RFC3339). */
export function zonedDateTimeToISO(date: string, time: string, timeZone: string): string {
  const [y, mo, d] = date.split("-").map(Number);
  const [hh, mm] = time.split(":").map(Number);
  let ms = Date.UTC(y, mo - 1, d, hh, mm, 0);
  for (let i = 0; i < 6; i++) {
    const f = formatInTimezone(ms, timeZone);
    const diffMin =
      hh * 60 +
      mm -
      (Number(f.time.slice(0, 2)) * 60 + Number(f.time.slice(3, 5))) +
      (d - Number(f.date.slice(8, 10))) * 24 * 60;
    if (diffMin === 0 && f.date === date) return new Date(ms).toISOString();
    ms += diffMin * 60_000;
  }
  return new Date(ms).toISOString();
}
