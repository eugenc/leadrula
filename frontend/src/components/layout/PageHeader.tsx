export function PageHeader({
  title,
  subtitle,
  action,
}: {
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="mb-5 flex items-end justify-between">
      <div>
        <h2 className="text-xl font-bold text-pd-text">{title}</h2>
        {subtitle && <p className="text-sm text-pd-muted">{subtitle}</p>}
      </div>
      {action}
    </div>
  );
}
