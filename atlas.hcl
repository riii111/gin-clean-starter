env "dev" {
  src = "file://migrations"
  dev = "docker://postgres/17/dev?search_path=public"
  url = env("DATABASE_URL")
  migration {
    dir = "file://migrations"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

env "local" {
  src = "file://migrations"
  dev = "docker://postgres/17/dev?search_path=public"
  url = "postgres://app:app@db:5432/app?sslmode=disable"
  migration {
    dir = "file://migrations"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
