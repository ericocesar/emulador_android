import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { Monitor, LogOut, User, Package, Key, Send, Smartphone } from 'lucide-react';
import { useAuth } from '../hooks/useAuth';

interface NavItem {
  to: string;
  label: string;
  icon: React.ReactNode;
}

const sectionOperacao: NavItem[] = [
  { to: '/dashboard',  label: 'Dispositivos',     icon: <Smartphone className="w-[18px] h-[18px]" /> },
  { to: '/campaigns',  label: 'Disparos',         icon: <Send className="w-[18px] h-[18px]" /> },
];

const sectionConfig: NavItem[] = [
  { to: '/apk', label: 'Aplicativo (APK)', icon: <Package className="w-[18px] h-[18px]" /> },
  { to: '/api', label: 'API',              icon: <Key className="w-[18px] h-[18px]" /> },
];

function NavLinkItem({ item }: { item: NavItem }) {
  return (
    <NavLink
      to={item.to}
      className={({ isActive }) =>
        `group flex items-center gap-3 px-3 py-2 rounded-lg text-[13px] font-medium transition-colors ${
          isActive
            ? 'bg-forest-900 text-white'
            : 'text-gray-600 hover:text-forest-900 hover:bg-gray-100/70'
        }`
      }
    >
      {item.icon}
      {item.label}
    </NavLink>
  );
}

export default function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  return (
    <div className="flex h-screen overflow-hidden bg-page">
      {/* Sidebar */}
      <aside className="w-60 bg-white border-r border-gray-200 flex flex-col flex-shrink-0">
        {/* Logo */}
        <div className="px-5 py-5 flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-lg bg-forest-900 grid place-items-center">
            <Monitor className="w-4 h-4 text-lime-accent" />
          </div>
          <div className="leading-tight">
            <div className="font-bold text-[15px] text-forest-950 tracking-tight">AstraDroid</div>
            <div className="text-[10px] text-gray-400 -mt-0.5">v0.0.1 · develop</div>
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-3 py-2 overflow-y-auto">
          <div className="px-3 mb-1.5 mt-2 section-label">Operação</div>
          <div className="space-y-0.5">
            {sectionOperacao.map(item => <NavLinkItem key={item.to} item={item} />)}
          </div>

          <div className="px-3 mb-1.5 mt-5 section-label">Configuração</div>
          <div className="space-y-0.5">
            {sectionConfig.map(item => <NavLinkItem key={item.to} item={item} />)}
          </div>
        </nav>

        {/* User chip */}
        <div className="p-3 border-t border-gray-200">
          <div className="rounded-xl bg-gray-50 border border-gray-200 px-3 py-2.5 flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-full bg-gradient-to-br from-forest-700 to-forest-900 flex items-center justify-center text-white shrink-0">
              <User className="w-4 h-4" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-[13px] font-semibold text-gray-900 truncate">{user?.name || 'Usuário'}</div>
              <div className="text-[11px] text-gray-500 truncate">{user?.email}</div>
            </div>
            <button
              onClick={handleLogout}
              title="Sair"
              className="p-1.5 rounded-md text-gray-400 hover:text-gray-700 hover:bg-white transition-colors"
            >
              <LogOut className="w-4 h-4" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto bg-page">
        <Outlet />
      </main>
    </div>
  );
}
