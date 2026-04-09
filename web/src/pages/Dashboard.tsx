import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import client from '@/api/client';
import { useAuth } from '@/context/AuthContext';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';

interface Counts {
  taxis: number;
  drivers: number;
  pending_expenses: number;
}

export default function Dashboard() {
  const [counts, setCounts] = useState<Counts | null>(null);
  const { logout } = useAuth();

  useEffect(() => {
    async function load() {
      const [taxisRes, driversRes, expensesRes] = await Promise.all([
        client.get<{ taxis: unknown[] }>('/taxis'),
        client.get<{ drivers: unknown[] }>('/drivers'),
        client.get<{ expenses: unknown[] }>('/expenses?status=confirmed'),
      ]);
      setCounts({
        taxis: taxisRes.data.taxis?.length ?? 0,
        drivers: driversRes.data.drivers?.length ?? 0,
        pending_expenses: expensesRes.data.expenses?.length ?? 0,
      });
    }
    load();
  }, []);

  return (
    <div className="min-h-screen p-8 max-w-4xl mx-auto">
      <div className="flex justify-between items-center mb-8">
        <h1 className="text-2xl font-bold">Panel de Control</h1>
        <Button variant="outline" onClick={logout}>Salir</Button>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
        <Card>
          <CardHeader><CardTitle className="text-sm">Taxis Activos</CardTitle></CardHeader>
          <CardContent><p className="text-3xl font-bold">{counts?.taxis ?? '…'}</p></CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-sm">Conductores Activos</CardTitle></CardHeader>
          <CardContent><p className="text-3xl font-bold">{counts?.drivers ?? '…'}</p></CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-sm">Gastos Pendientes</CardTitle></CardHeader>
          <CardContent><p className="text-3xl font-bold">{counts?.pending_expenses ?? '…'}</p></CardContent>
        </Card>
      </div>

      <nav className="flex flex-col gap-2">
        <Link to="/taxis"><Button variant="outline" className="w-full justify-start">Taxis</Button></Link>
        <Link to="/drivers"><Button variant="outline" className="w-full justify-start">Conductores</Button></Link>
        <Link to="/expenses"><Button variant="outline" className="w-full justify-start">Gastos</Button></Link>
        <Link to="/reports"><Button variant="outline" className="w-full justify-start">Reportes</Button></Link>
      </nav>
    </div>
  );
}
