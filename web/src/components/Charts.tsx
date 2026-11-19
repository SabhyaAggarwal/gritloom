import type { DailyActivity, Project } from '../types';
import { formatDuration, formatDayDate, localDateKey } from '../utils';

export function WeeklyChart({ daily }: { daily: DailyActivity[] }) {
  const days = Array.from({ length: 7 }, (_, index) => {
    const date = new Date();
    date.setDate(date.getDate() - (6 - index));
    const key = localDateKey(date);
    const match = daily.find((item) => item.date.slice(0, 10) === key);
    return match ?? { date: key, seconds: 0, ai_seconds: 0 };
  });
  const width = 700;
  const height = 258;
  const top = 18;
  const bottom = 43;
  const plotHeight = height - top - bottom;
  const slot = width / 7;
  const barWidth = 44;
  const maximum = Math.max(3600, ...days.map((day) => day.seconds + day.ai_seconds));
  const roundedMaximum = Math.ceil(maximum / 3600) * 3600;
  const ticks = [0, 0.5, 1];
  const labelFor = (date: string, format: 'narrow' | 'short') => {
    const parsed = new Date(`${date.slice(0, 10)}T12:00:00`);
    return new Intl.DateTimeFormat(undefined, { weekday: format }).format(parsed);
  };

  return (
    <div className="weekly-chart">
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-labelledby="week-chart-title week-chart-description">
        <title id="week-chart-title">Engaged and background AI time over the last seven days</title>
        <desc id="week-chart-description">
          {days.map((day) => `${formatDayDate(day.date)}: ${formatDuration(day.seconds)} engaged and ${formatDuration(day.ai_seconds)} background AI`).join('. ')}
        </desc>
        {ticks.map((tick) => {
          const y = top + plotHeight * (1 - tick);
          return (
            <g key={tick} aria-hidden="true">
              <line x1="0" x2={width} y1={y} y2={y} className="chart-gridline" />
              <text x="0" y={y - 7} className="chart-y-label">{tick === 0 ? '0' : `${Math.round(roundedMaximum * tick / 3600)}h`}</text>
            </g>
          );
        })}
        {days.map((day, index) => {
          const center = slot * index + slot / 2;
          const engagedHeight = day.seconds / roundedMaximum * plotHeight;
          const aiHeight = day.ai_seconds / roundedMaximum * plotHeight;
          const base = top + plotHeight;
          const today = index === 6;
          return (
            <g key={day.date}>
              {today && <rect x={center - slot / 2 + 5} y={top - 9} width={slot - 10} height={height - top - 3} rx="3" className="chart-today" aria-hidden="true" />}
              <rect x={center - barWidth / 2} y={base - engagedHeight} width={barWidth} height={Math.max(day.seconds > 0 ? 3 : 0, engagedHeight)} rx="3" className="chart-engaged" aria-hidden="true" />
              {day.ai_seconds > 0 && <rect x={center - barWidth / 2} y={base - engagedHeight - aiHeight - 3} width={barWidth} height={Math.max(3, aiHeight)} rx="3" className="chart-ai" aria-hidden="true" />}
              <text x={center} y={height - 19} textAnchor="middle" className={`chart-day ${today ? 'chart-day-today' : ''}`} aria-hidden="true">
                {labelFor(day.date, 'narrow')}
              </text>
              <text x={center} y={height - 5} textAnchor="middle" className="chart-date" aria-hidden="true">
                {Number(day.date.slice(8, 10))}
              </text>
            </g>
          );
        })}
      </svg>
      <div className="chart-legend" aria-hidden="true">
        <span><i className="legend-swatch swatch-engaged" />Engaged making</span>
        <span><i className="legend-swatch swatch-ai" />Background AI</span>
      </div>
      <table className="sr-only">
        <caption>Daily engaged and background AI time</caption>
        <thead><tr><th>Day</th><th>Engaged</th><th>Background AI</th></tr></thead>
        <tbody>{days.map((day) => <tr key={day.date}><th>{formatDayDate(day.date)}</th><td>{formatDuration(day.seconds)}</td><td>{formatDuration(day.ai_seconds)}</td></tr>)}</tbody>
      </table>
    </div>
  );
}

export function ProjectAllocation({ projects, limit = 5 }: { projects: Project[]; limit?: number }) {
  const sorted = [...projects].sort((a, b) => b.total_seconds - a.total_seconds);
  const visible = sorted.slice(0, limit);
  const total = sorted.reduce((sum, project) => sum + project.total_seconds, 0);
  if (!projects.length || total <= 0) return null;

  return (
    <div className="allocation">
      <div className="allocation-bar" role="img" aria-label={`Project allocation: ${visible.map((project) => `${project.name} ${Math.round(project.total_seconds / total * 100)} percent`).join(', ')}`}>
        {visible.map((project) => (
          <span key={project.id} style={{ width: `${project.total_seconds / total * 100}%`, backgroundColor: project.color }} title={`${project.name}: ${Math.round(project.total_seconds / total * 100)}%`} />
        ))}
      </div>
      <ol className="allocation-list">
        {visible.map((project) => (
          <li key={project.id}>
            <span className="project-dot" style={{ backgroundColor: project.color }} aria-hidden="true" />
            <span className="allocation-name">{project.name}</span>
            <span className="allocation-language">{project.language || 'Project'}</span>
            <strong>{formatDuration(project.total_seconds)}</strong>
            <span className="allocation-percent">{Math.round(project.total_seconds / total * 100)}%</span>
          </li>
        ))}
      </ol>
    </div>
  );
}
