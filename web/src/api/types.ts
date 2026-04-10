export interface Owner {
  id: string;
  name: string;
  email: string;
  created_at: string;
}

export interface Taxi {
  id: string;
  owner_id: string;
  plate: string;
  model: string;
  year: number;
  active: boolean;
  created_at: string;
}

export interface Driver {
  id: string;
  owner_id: string;
  name: string;
  phone: string;
  telegram_id: number | null;
  active: boolean;
  created_at: string;
}

export interface Expense {
  id: string;
  owner_id: string;
  driver_id: string;
  taxi_id: string;
  driver_name: string;
  taxi_plate: string;
  category: string;
  amount: string;
  status: 'pending' | 'confirmed' | 'approved' | 'rejected';
  notes: string;
  reject_reason: string;
  receipt_image_url: string;
  ocr_text: string;
  created_at: string;
}

export interface Report {
  taxi_plate?: string;
  driver_name?: string;
  total: string;
}
