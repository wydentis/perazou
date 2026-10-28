export interface Group {
  id: number;
  name: string;
  timezone: string;
}
export interface Member {
  user_id: number;
  full_name: string;
  role: 'headman' | 'student';
  username?: string;
}
export interface Me {
  user: { id: number; first_name: string };
  member: Member | null;
  group: Group | null;
  is_admin: boolean;
}
export interface AdminUser {
  id: number;
  username: string;
  first_name: string;
  last_name: string;
  full_name: string;
  role: '' | 'headman' | 'student';
  group_id: number | null;
  group_name: string;
  created_at: string;
  joined_at: string | null;
}
export interface AdminGroup {
  id: number;
  name: string;
  headman: string;
  members: number;
  slots: number;
  marks: number;
  created_at: string;
}
export interface Slot {
  id: number;
  weekday: number;
  start_time: string;
  end_time: string;
  subject: string;
  room: string;
  teacher: string;
  valid_from: string; // YYYY-MM-DD or ''
  valid_to: string;
}
export type Status = 'present' | 'absent' | '';
export interface Lesson extends Slot {
  date: string;
  cancelled: boolean;
  started: boolean;
  open: boolean;
  opens_at: string;
  closes_at: string;
  my_status: Status;
  my_auto: boolean;
  present: number;
  absent: number;
  auto_absent: number;
  total: number;
}
export interface AttendanceRow {
  user_id: number;
  full_name: string;
  status: Status;
  auto: boolean; // "absent" set automatically after the marking window closed
}
export interface StatRow {
  user_id: number;
  full_name: string;
  present: number;
  absent: number;
  unmarked: number;
  total: number;
}
export interface Stats {
  from: string;
  to: string;
  lessons: number;
  rows: StatRow[];
}
export interface ImportLesson {
  weekday: number;
  start_time: string;
  end_time: string;
  subject: string;
  sub: '' | 'А' | 'Б';
  teacher_a: string;
  teacher_b: string;
  room_a: string;
  room_b: string;
  valid_from: string;
  valid_to: string;
  note: string;
}
export interface ImportGroup {
  name: string;
  lessons: ImportLesson[];
}
export interface GroupDetails {
  group: Group;
  members: Member[];
  invite_code?: string;
  invite_link?: string;
}
