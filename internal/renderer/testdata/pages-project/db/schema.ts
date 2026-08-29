import { pgTable, serial, text } from "drizzle-orm/pg-core"

export const comments = pgTable("comments", {
  id: serial("id").primaryKey(),
  content: text("content").notNull(),
})
