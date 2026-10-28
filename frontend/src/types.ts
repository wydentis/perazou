export interface Group {
  id: number;
  name: string;
  timezone: string;
  week1_monday: string;
  current_week: 'odd' | 'even';
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
}
export interface Slot {
  id: number;
  weekday: number;
  start_time: string;
  end_time: string;
  subject: string;
  room: string;
  parity: 0 | 1 | 2;
}
export type Status = 'present' | 'absent' | '';
export interface Lesson extends Slot {
  date: string;
  cancelled: boolean;
  started: boolean;
  open: boolean;
  my_status: Status;
  present: number;
  absent: number;
  total: number;
}
export interface AttendanceRow {
  user_id: number;
  full_name: string;
  status: Status;
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
export interface GroupDetails {
  group: Group;
  members: Member[];
  invite_code?: string;
  invite_link?: string;
}
